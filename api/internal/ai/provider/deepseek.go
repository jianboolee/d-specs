package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.deepseek.com"
	DefaultModel   = "deepseek-chat"
)

// DeepSeek 基于 OpenAI 兼容协议的 DeepSeek 客户端
type DeepSeek struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

type DeepSeekOptions struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

func NewDeepSeek(opt DeepSeekOptions) (*DeepSeek, error) {
	if strings.TrimSpace(opt.APIKey) == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY 未配置")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(opt.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	model := strings.TrimSpace(opt.Model)
	if model == "" {
		model = DefaultModel
	}

	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	return &DeepSeek{
		apiKey:  opt.APIKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func (d *DeepSeek) Name() string  { return "deepseek" }
func (d *DeepSeek) Model() string { return d.model }

func (d *DeepSeek) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body := d.buildBody(req)

	var out chatCompletionResponse
	if err := d.do(ctx, http.MethodPost, "/chat/completions", body, &out); err != nil {
		return nil, err
	}

	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("deepseek 返回空 choices（finish_reason: %s）", out.ErrorMessage())
	}

	choice := out.Choices[0]
	return &ChatResponse{
		Message:      choice.Message,
		FinishReason: choice.FinishReason,
		Usage:        out.Usage.toUsage(),
	}, nil
}

func (d *DeepSeek) buildBody(req ChatRequest) map[string]any {
	body := map[string]any{
		"model":    d.model,
		"messages": req.Messages,
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	return body
}

// ChatStream 以流式方式执行对话。
//
// 只在「尚未开始收到数据」的阶段做重试：一旦已经吐出部分增量，
// 重试会导致前端收到重复内容，因此那之后的错误直接上抛。
func (d *DeepSeek) ChatStream(
	ctx context.Context,
	req ChatRequest,
	onDelta func(StreamDelta),
) (*ChatResponse, error) {
	body := d.buildBody(req)
	body["stream"] = true
	// 流式默认不带 usage，显式要求服务端在最后一块里带上
	body["stream_options"] = map[string]any{"include_usage": true}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt-1) * time.Second):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("构造请求失败: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := d.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("请求 deepseek 失败: %w", err)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			_ = resp.Body.Close()
			apiErr := &APIError{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
			var envelope struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(respBody, &envelope) == nil && envelope.Error.Message != "" {
				apiErr.Code = envelope.Error.Code
				apiErr.Message = envelope.Error.Message
			}
			if !apiErr.IsRetryable() {
				return nil, apiErr
			}
			lastErr = apiErr
			continue
		}

		return d.consumeStream(resp.Body, onDelta)
	}

	return nil, lastErr
}

// consumeStream 解析 SSE 流，边回调增量边拼出完整响应。
func (d *DeepSeek) consumeStream(body io.ReadCloser, onDelta func(StreamDelta)) (*ChatResponse, error) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	// 单个 SSE 事件行可能较大，放宽上限；默认 64KB 在长工具参数下会被截断
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var content strings.Builder
	var reasoning strings.Builder
	acc := newToolCallAccumulator()
	finish := ""
	var usage Usage

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			// 空行是事件分隔；以 : 开头是注释/心跳
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// 单块解析失败不该中断整段流，跳过即可
			continue
		}

		if chunk.Usage != nil {
			usage = chunk.Usage.toUsage()
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		if delta := choice.Delta.Content; delta != "" {
			content.WriteString(delta)
			if onDelta != nil {
				onDelta(StreamDelta{Content: delta})
			}
		}
		if delta := choice.Delta.ReasoningContent; delta != "" {
			reasoning.WriteString(delta)
			if onDelta != nil {
				onDelta(StreamDelta{Reasoning: delta})
			}
		}
		acc.add(choice.Delta.ToolCalls)
		if choice.FinishReason != "" {
			finish = choice.FinishReason
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取流式响应失败: %w", err)
	}

	return &ChatResponse{
		Message: Message{
			Role:             RoleAssistant,
			Content:          content.String(),
			ReasoningContent: reasoning.String(),
			ToolCalls:        acc.result(),
		},
		FinishReason: finish,
		Usage:        usage,
	}, nil
}

func (d *DeepSeek) ListModels(ctx context.Context) ([]string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := d.do(ctx, http.MethodGet, "/models", nil, &out); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

type chatCompletionResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	// usage 是嵌套结构（缓存命中与思考 token 在子对象里），不能直接映射到扁平的 Usage
	Usage usageResponse `json:"usage"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// streamChunk 流式响应里的单个 SSE 数据块。
//
// 工具调用是分片到达的：先给 id 与函数名，参数再逐段拼接，因此
// delta.tool_calls 需要用 index 归并（见 toolCallAccumulator）。
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string          `json:"content"`
			ReasoningContent string          `json:"reasoning_content"`
			ToolCalls        []toolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *usageResponse `json:"usage"`
}

type toolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// toolCallAccumulator 按 index 归并流式到达的工具调用分片。
type toolCallAccumulator struct {
	byIndex map[int]*ToolCall
}

func newToolCallAccumulator() *toolCallAccumulator {
	return &toolCallAccumulator{byIndex: map[int]*ToolCall{}}
}

func (a *toolCallAccumulator) add(deltas []toolCallDelta) {
	for _, d := range deltas {
		call, ok := a.byIndex[d.Index]
		if !ok {
			call = &ToolCall{Type: ToolCallTypeFunction}
			a.byIndex[d.Index] = call
		}
		// id 与函数名通常只在第一片出现，后续分片只带 arguments
		if d.ID != "" {
			call.ID = d.ID
		}
		if d.Type != "" {
			call.Type = d.Type
		}
		if d.Function.Name != "" {
			call.Function.Name = d.Function.Name
		}
		call.Function.Arguments += d.Function.Arguments
	}
}

// result 按 index 升序返回，保证工具调用顺序与模型输出一致。
func (a *toolCallAccumulator) result() []ToolCall {
	if len(a.byIndex) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(a.byIndex))
	for i := range a.byIndex {
		indexes = append(indexes, i)
	}
	sort.Ints(indexes)

	out := make([]ToolCall, 0, len(indexes))
	for _, i := range indexes {
		out = append(out, *a.byIndex[i])
	}
	return out
}

type usageResponse struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

func (u usageResponse) toUsage() Usage {
	return Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		CachedTokens:     u.PromptTokensDetails.CachedTokens,
		ReasoningTokens:  u.CompletionTokensDetails.ReasoningTokens,
	}
}

func (r chatCompletionResponse) ErrorMessage() string {
	if r.Error != nil {
		return r.Error.Message
	}
	return "unknown"
}

// do 发起请求，对限流与服务端错误做有限重试。
//
// 只接 Flash 档位时模型侧抖动会更明显，重试能挡掉相当一部分偶发失败。
func (d *DeepSeek) do(ctx context.Context, method, path string, payload, out any) error {
	var bodyBytes []byte
	if payload != nil {
		var err error
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("序列化请求失败: %w", err)
		}
	}

	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt-1) * time.Second):
			}
		}

		var reader io.Reader
		if bodyBytes != nil {
			reader = bytes.NewReader(bodyBytes)
		}

		httpReq, err := http.NewRequestWithContext(ctx, method, d.baseURL+path, reader)
		if err != nil {
			return fmt.Errorf("构造请求失败: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
		httpReq.Header.Set("Accept", "application/json")

		resp, err := d.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("请求 deepseek 失败: %w", err)
			continue
		}

		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("读取响应失败: %w", readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("解析响应失败: %w (body: %s)", err, truncate(string(respBody), 500))
			}
			return nil
		}

		apiErr := &APIError{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &envelope) == nil && envelope.Error.Message != "" {
			apiErr.Code = envelope.Error.Code
			apiErr.Message = envelope.Error.Message
		}

		if !apiErr.IsRetryable() {
			return apiErr
		}
		lastErr = apiErr
	}

	return lastErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
