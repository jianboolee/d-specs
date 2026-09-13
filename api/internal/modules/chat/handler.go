package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jianboolee/d-specs/internal/contextx"
	"github.com/jianboolee/d-specs/internal/modules/analysis"
	"github.com/jianboolee/d-specs/internal/modules/spec"
	"github.com/jianboolee/d-specs/internal/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Stream POST /api/v1/tables/:id/chat —— 以 SSE 流式返回整轮对话。
func (h *Handler) Stream(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	tableID := c.Param("id")
	ctx := c.Request.Context()

	// 前置校验放在写 SSE 头之前：这样「表不存在」「消息为空」「AI 未配置」
	// 这类错误仍然能以正常的 JSON 响应返回，而不是给前端一段半截的流
	if err := h.svc.Prepare(ctx, userID, tableID, req.Message); err != nil {
		writeError(c, err)
		return
	}

	emit, ok := startSSE(c, ctx)
	if !ok {
		return
	}
	runSSE(ctx, emit, func(push Emit) error {
		return h.svc.Stream(ctx, userID, tableID, req, push)
	})
}

// Analyze POST /api/v1/tables/:id/analyze
//
// 与聊天同一条 SSE 通道：「完善参数表」对用户来说就是一轮对话。
// 前置校验同样必须在写 SSE 头之前完成。
func (h *Handler) Analyze(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	tableID := c.Param("id")
	ctx := c.Request.Context()

	if err := h.svc.PrepareAnalyze(ctx, userID, tableID); err != nil {
		writeError(c, err)
		return
	}

	emit, ok := startSSE(c, ctx)
	if !ok {
		return
	}
	runSSE(ctx, emit, func(push Emit) error {
		return h.svc.Analyze(ctx, userID, tableID, req, push)
	})
}

// startSSE 写好 SSE 响应头并返回 emit 闭包；底层不支持 flush 时返回 false。
func startSSE(c *gin.Context, ctx context.Context) (Emit, bool) {
	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// 关掉 nginx 的响应缓冲，否则事件会被攒到最后一起发，流式就白做了
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, false
	}
	flusher.Flush()

	emit := func(ev Event) error {
		payload, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", ev.Type, payload); err != nil {
			return err
		}
		flusher.Flush()
		return ctx.Err()
	}
	return emit, true
}

// runSSE 执行流式回调，并把中途的失败转成一个 error 事件推给前端。
func runSSE(ctx context.Context, emit Emit, run func(Emit) error) {
	streamErr := run(emit)
	if streamErr == nil {
		return
	}

	// 客户端主动断开是常态，此时连接已不可写，再发事件没有意义
	if ctx.Err() != nil {
		return
	}

	_ = emit(Event{Type: EventError, Message: describeError(streamErr)})
}

func (h *Handler) Sessions(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Sessions(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Messages(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Messages(c.Request.Context(), userID, c.Param("sid"), 100)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

// ==================== 辅助 ====================

func requireUser(c *gin.Context) (string, bool) {
	userID, err := contextx.RequireUserID(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusUnauthorized, 401, "请先登录")
		return "", false
	}
	return userID, true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrLLMUnavailable):
		response.Error(c, http.StatusServiceUnavailable, 503, err.Error())
	case errors.Is(err, ErrSessionNotFound):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, ErrEmptyMessage):
		response.Error(c, http.StatusBadRequest, 400, err.Error())
	case errors.Is(err, analysis.ErrNothingToAnalyze):
		response.Error(c, http.StatusBadRequest, 400, err.Error())
	case errors.Is(err, spec.ErrTableNotFound):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, spec.ErrRevisionConflict):
		response.Error(c, http.StatusConflict, 409, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, 500, "服务器内部错误")
	}
}

// describeError 把内部错误转成可以直接展示给用户的提示
func describeError(err error) string {
	switch {
	case errors.Is(err, ErrLLMUnavailable):
		return err.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "AI 响应超时，请稍后重试"
	case errors.Is(err, spec.ErrRevisionConflict):
		return "参数表刚刚被其他操作改动，请重试"
	default:
		return "AI 处理失败：" + err.Error()
	}
}
