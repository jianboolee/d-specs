// Command aicheck 是 Phase 0 的 AI 连通性验证工具。
//
// 为什么要在 Phase 0 就做这件事：已经确定只接 Flash 档位、且首版要做到 Phase 2。
// 那么「Flash 的 function calling 是否稳定」就是整个 Phase 2 的前提假设。
// 把这个假设提前证伪或证实，比做到一半才发现要换方案便宜得多。
//
// 用法：
//
//	cp .env.example .env   # 填入 DEEPSEEK_API_KEY
//	go run ./cmd/aicheck
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/donktech/d-specs/internal/ai/agent"
	"github.com/donktech/d-specs/internal/ai/ops"
	"github.com/donktech/d-specs/internal/ai/provider"
	"github.com/donktech/d-specs/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalf("加载配置失败: %v", err)
	}

	fmt.Println("=== 1. 配置检查 ===")
	fmt.Printf("  APP_ENV:  %s\n", cfg.AppEnv)
	fmt.Printf("  base_url: %s\n", cfg.AI.DeepSeek.BaseURL)
	fmt.Printf("  model:    %s\n", cfg.AI.DeepSeek.Model)
	if cfg.AI.DeepSeek.APIKey == "" {
		fmt.Println("  api_key:  【未配置】")
		fmt.Println()
		fmt.Println("请先在 api/.env 中填入 DEEPSEEK_API_KEY 后重试：")
		fmt.Println("  cp .env.example .env  &&  vi .env")
		os.Exit(1)
	}
	fmt.Printf("  api_key:  %s\n", maskKey(cfg.AI.DeepSeek.APIKey))

	llm, err := provider.NewDeepSeek(provider.DeepSeekOptions{
		APIKey:  cfg.AI.DeepSeek.APIKey,
		BaseURL: cfg.AI.DeepSeek.BaseURL,
		Model:   cfg.AI.DeepSeek.Model,
		Timeout: cfg.AI.DeepSeek.Timeout,
	})
	if err != nil {
		fatalf("初始化 DeepSeek 客户端失败: %v", err)
	}

	failures := 0

	// ---------- 2. 模型列表 ----------
	fmt.Println("\n=== 2. 可用模型列表 ===")
	if models, err := llm.ListModels(context.Background()); err != nil {
		fmt.Printf("  ✗ 拉取失败: %v\n", err)
		fmt.Println("    （若接口不可用可忽略，不影响后续验证）")
	} else {
		fmt.Printf("  可用模型: %s\n", strings.Join(models, ", "))
		current := llm.Model()
		found := false
		for _, m := range models {
			if m == current {
				found = true
				break
			}
		}
		if found {
			fmt.Printf("  ✓ 当前配置的模型 %s 在列表中\n", current)
		} else {
			fmt.Printf("  ⚠ 当前配置的模型 %s 不在列表中，请核对 DEEPSEEK_MODEL\n", current)
		}
	}

	// ---------- 3. 基础对话 ----------
	fmt.Println("\n=== 3. 基础对话 ===")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	resp, err := llm.Chat(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "只回复两个字：在的"},
		},
		Temperature: 0,
		MaxTokens:   cfg.AI.MaxTokens,
	})
	cancel()
	if err != nil {
		fmt.Printf("  ✗ 调用失败: %v\n", err)
		failures++
	} else {
		reply := strings.TrimSpace(resp.Message.Content)
		if reply == "" {
			fmt.Printf("  ✗ 正文为空（finish_reason=%s）\n", resp.FinishReason)
			fmt.Println("    推理模型下这通常意味着 max_tokens 不够，思考把预算吃光了")
			failures++
		} else {
			fmt.Printf("  ✓ 回复: %q\n", reply)
		}
		fmt.Printf("    usage: prompt=%d completion=%d（其中思考 %d）%s\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.ReasoningTokens, cachedNote(resp.Usage))
		if resp.Message.ReasoningContent != "" {
			fmt.Printf("    模型返回了 reasoning_content：这是推理模型，思考不展示给用户\n")
		}
	}

	// ---------- 4. Function calling 基础能力 ----------
	fmt.Println("\n=== 4. Function calling 基础能力 ===")
	if !checkToolCalling(cfg, llm) {
		failures++
	}

	// ---------- 5. Agent 全链路 ----------
	fmt.Println("\n=== 5. Agent 全链路（真实工具 + 校验 + 应用）===")
	for _, sc := range scenarios() {
		if !runScenario(cfg, llm, sc) {
			failures++
		}
	}

	// ---------- 结论 ----------
	fmt.Println("\n=== 结论 ===")
	if failures == 0 {
		fmt.Println("  ✓ 全部通过。Flash 的 function calling 可以直接支撑 Phase 2 的方案。")
		os.Exit(0)
	}
	fmt.Printf("  ✗ 有 %d 项未通过，请查看上方输出。\n", failures)
	fmt.Println("    若 function calling 不稳定，退路是改用 JSON 输出模式 + 服务端 schema 校验，")
	fmt.Println("    两者共用同一套 ops 校验器，切换成本主要在 provider 层。")
	os.Exit(1)
}

// checkToolCalling 验证模型能否按 schema 正确调用工具
func checkToolCalling(cfg *config.Config, llm provider.Provider) bool {
	demoTool := provider.NewFunctionTool(
		"record_phone_spec",
		"记录一条手机参数。当用户提到手机参数时使用。",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param_name": map[string]any{"type": "string", "description": "参数名，例如「电池容量」"},
				"value":      map[string]any{"type": "string", "description": "参数值，单位直接写在值里，例如 5000mAh"},
			},
			"required": []string{"param_name", "value"},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := llm.Chat(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "你必须通过工具来记录参数，不要用文字回答。"},
			{Role: provider.RoleUser, Content: "这台手机电池是 5000mAh"},
		},
		Tools:       []provider.Tool{demoTool},
		Temperature: 0,
	})
	if err != nil {
		fmt.Printf("  ✗ 调用失败: %v\n", err)
		return false
	}

	calls := resp.ToolCalls()
	if len(calls) == 0 {
		fmt.Printf("  ✗ 模型没有返回 tool_call，而是直接回复: %q\n", strings.TrimSpace(resp.Message.Content))
		fmt.Println("    这通常意味着该模型档位的 function calling 支持不足")
		return false
	}

	call := calls[0]
	if call.Function.Name != "record_phone_spec" {
		fmt.Printf("  ✗ 工具名不符，期望 record_phone_spec，实际 %s\n", call.Function.Name)
		return false
	}

	args, err := call.Function.ParseArguments()
	if err != nil {
		fmt.Printf("  ✗ 参数解析失败: %v (原始: %s)\n", err, call.Function.Arguments)
		return false
	}

	pretty, _ := json.Marshal(args)
	fmt.Printf("  ✓ 正确调用工具，参数: %s\n", string(pretty))

	// 关键校验：值里必须带单位，不能被拆成「数字 + 单位」两个字段
	value := strings.TrimSpace(fmt.Sprintf("%v", args["value"]))
	if strings.ReplaceAll(value, " ", "") == "5000mAh" {
		fmt.Println("  ✓ 参数值完整含单位（value=5000mAh）")
		return true
	}
	fmt.Printf("  ⚠ 参数值不理想（value=%q），期望 value=5000mAh\n", value)
	fmt.Println("    提示：单位被拆出去会丢信息，需要靠 system prompt 与工具 schema 兜住")
	return false
}

type scenario struct {
	name   string
	table  ops.Snapshot
	prompt string
	verify func(res *agent.RunResult) error
}

func scenarios() []scenario {
	return []scenario{
		{
			name:   "空表建表",
			table:  ops.Snapshot{Title: "新建配置表"},
			prompt: "帮我建一张蓝牙耳机的配置表，要有蓝牙版本、续航时间、单耳重量、充电接口这几个参数",
			verify: func(res *agent.RunResult) error {
				if !res.Applied {
					return fmt.Errorf("模型没有产生任何变更（reply: %s）", oneLine(res.Reply))
				}
				added := 0
				for _, c := range res.Changes {
					if c.Kind == ops.ChangeAdd {
						added++
					}
				}
				if added < 4 {
					return fmt.Errorf("期望新增至少 4 个参数项，实际 %d 个", added)
				}
				return nil
			},
		},
		{
			name:   "按名称改值（值含单位）",
			table:  sampleTable(),
			prompt: "电池容量改成 5000mAh",
			verify: func(res *agent.RunResult) error {
				if !res.Applied {
					return fmt.Errorf("模型没有产生变更（reply: %s）", oneLine(res.Reply))
				}
				for _, it := range res.Snapshot.Items {
					if it.Name != "电池容量" {
						continue
					}
					// 单位与数值是一个整体，不该被拆开
					if strings.ReplaceAll(it.Value, " ", "") != "5000mAh" {
						return fmt.Errorf("期望 value=5000mAh，实际 value=%q", it.Value)
					}
					return nil
				}
				return fmt.Errorf("变更后找不到「电池容量」这一项")
			},
		},
		{
			name:   "批量条件式改动",
			table:  sampleTable(),
			prompt: "把续航分组下所有参数的数值都翻倍，各自的单位保持不变",
			verify: func(res *agent.RunResult) error {
				if !res.Applied {
					return fmt.Errorf("模型没有产生变更（reply: %s）", oneLine(res.Reply))
				}
				// 续航组原有 电池容量=4800mAh、充电功率=30W
				for _, it := range res.Snapshot.Items {
					switch it.Name {
					case "电池容量":
						if strings.ReplaceAll(it.Value, " ", "") != "9600mAh" {
							return fmt.Errorf("电池容量期望 9600mAh，实际 %s", it.Value)
						}
					case "充电功率":
						if strings.ReplaceAll(it.Value, " ", "") != "60W" {
							return fmt.Errorf("充电功率期望 60W，实际 %s", it.Value)
						}
					}
				}
				return nil
			},
		},
		{
			name:   "只读提问不应改数据",
			table:  sampleTable(),
			prompt: "现在表里有哪些参数？",
			verify: func(res *agent.RunResult) error {
				if res.Applied {
					return fmt.Errorf("只是提问，不应该产生变更，但实际改了 %d 处", len(res.Changes))
				}
				if strings.TrimSpace(res.Reply) == "" {
					return fmt.Errorf("没有给出任何文字回复")
				}
				return nil
			},
		},
	}
}

func runScenario(cfg *config.Config, llm provider.Provider, sc scenario) bool {
	fmt.Printf("\n  ── 场景：%s\n", sc.name)
	fmt.Printf("     表：%s（%d 项）\n", sc.table.Title, len(sc.table.Items))
	fmt.Printf("     提问：%s\n", sc.prompt)

	ag := agent.New(llm, agent.Config{
		MaxToolRounds:   cfg.AI.MaxToolRounds,
		AutoApplyMaxOps: cfg.AI.AutoApplyMaxOps,
		MaxTokens:       cfg.AI.MaxTokens,
		Temperature:     cfg.AI.Temperature,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	start := time.Now()
	res, err := ag.Run(ctx, []provider.Message{
		{Role: provider.RoleUser, Content: sc.prompt},
	}, sc.table, func(ev agent.Event) {
		switch ev.Type {
		case agent.EventToolCall:
			fmt.Printf("     → 调用工具 %s %s\n", ev.ToolName, truncate(ev.ToolArgs, 160))
		case agent.EventApplied:
			fmt.Printf("     ✓ 已应用 %d 处变更：%s\n", len(ev.Changes), ev.Summary)
		case agent.EventPending:
			fmt.Printf("     ! 转待确认：%s\n", ev.Message)
		}
	})
	if err != nil {
		fmt.Printf("  ✗ 执行失败: %v\n", err)
		return false
	}

	fmt.Printf("     耗时 %s，轮数 %d，usage prompt=%d completion=%d（其中思考 %d）%s\n",
		time.Since(start).Round(time.Millisecond), res.Rounds,
		res.Usage.PromptTokens, res.Usage.CompletionTokens, res.Usage.ReasoningTokens,
		cachedNote(res.Usage))
	if res.Truncated {
		fmt.Printf("     ⚠ 达到工具轮数上限而中断\n")
	}
	fmt.Printf("     回复：%s\n", oneLine(res.Reply))

	if err := sc.verify(res); err != nil {
		fmt.Printf("  ✗ %s：%v\n", sc.name, err)
		return false
	}
	fmt.Printf("  ✓ %s 通过\n", sc.name)
	return true
}

// sampleTable 一份用于验证的示例配置表
func sampleTable() ops.Snapshot {
	return ops.Snapshot{
		Title: "示例手机 A1",
		Items: []ops.Item{
			{ID: "itm001", Name: "机身颜色", Value: "钛金属", ValueType: ops.TypeText, GroupName: "外观", SortOrder: 0},
			{ID: "itm002", Name: "重量", Value: "198g", ValueType: ops.TypeNumber, GroupName: "外观", SortOrder: 1},
			{ID: "itm003", Name: "处理器", Value: "A18", ValueType: ops.TypeText, GroupName: "性能", SortOrder: 2},
			{ID: "itm004", Name: "电池容量", Value: "4800mAh", ValueType: ops.TypeNumber, GroupName: "续航", SortOrder: 3},
			{ID: "itm005", Name: "充电功率", Value: "30W", ValueType: ops.TypeNumber, GroupName: "续航", SortOrder: 4},
		},
	}
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func oneLine(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	return truncate(s, 200)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func fatalf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
	os.Exit(1)
}

// cachedNote 展示前缀缓存命中情况。
// agent loop 每轮都会重发增长中的前缀，缓存命中率直接决定成本。
func cachedNote(u provider.Usage) string {
	if u.PromptTokens == 0 {
		return ""
	}
	pct := u.CachedTokens * 100 / u.PromptTokens
	return fmt.Sprintf("，前缀缓存命中 %d/%d (%d%%)", u.CachedTokens, u.PromptTokens, pct)
}
