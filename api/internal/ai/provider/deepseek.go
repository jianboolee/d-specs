package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
