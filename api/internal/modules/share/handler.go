package share

import (
	"errors"
	"net/http"

	"github.com/donktech/d-specs/internal/contextx"
	"github.com/donktech/d-specs/internal/modules/spec"
	"github.com/donktech/d-specs/internal/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Status(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Status(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Enable(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Enable(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Disable(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Disable(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

// PublicSpec 公开只读内容，无需登录。
func (h *Handler) PublicSpec(c *gin.Context) {
	res, err := h.svc.PublicSpec(c.Request.Context(), c.Param("token"))
	if err != nil {
		writeError(c, err)
		return
	}

	// 分享页不该被搜索引擎收录
	c.Header("X-Robots-Tag", "noindex, nofollow")
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
	case errors.Is(err, ErrShareNotFound), errors.Is(err, ErrNotShared):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, spec.ErrTableNotFound):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, 500, "服务器内部错误")
	}
}
