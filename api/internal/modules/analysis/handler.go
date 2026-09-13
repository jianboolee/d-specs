package analysis

import (
	"context"
	"errors"
	"net/http"

	"github.com/jianboolee/d-specs/internal/contextx"
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

// Analyze POST /api/v1/tables/:id/analyze
//
// 一次性返回分析结果（非流式）。标签已在服务端落库，建议只用于展示。
func (h *Handler) Analyze(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Analyze(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

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
	case errors.Is(err, ErrNothingToAnalyze):
		response.Error(c, http.StatusBadRequest, 400, err.Error())
	case errors.Is(err, spec.ErrTableNotFound):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(c, http.StatusGatewayTimeout, 504, "AI 响应超时，请稍后重试")
	default:
		response.Error(c, http.StatusInternalServerError, 500, "服务器内部错误")
	}
}
