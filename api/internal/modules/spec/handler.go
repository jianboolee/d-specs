package spec

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/jianboolee/d-specs/internal/contextx"
	"github.com/jianboolee/d-specs/internal/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	res, err := h.svc.List(c.Request.Context(), userID, c.Query("keyword"), page, size)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Create(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	res, err := h.svc.Create(c.Request.Context(), userID, req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Created(c, res)
}

func (h *Handler) Detail(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	res, err := h.svc.Detail(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Update(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	res, err := h.svc.UpdateMeta(c.Request.Context(), userID, c.Param("id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Delete(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), userID, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *Handler) Duplicate(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	var req DuplicateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	res, err := h.svc.Duplicate(c.Request.Context(), userID, c.Param("id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Created(c, res)
}

func (h *Handler) Apply(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	var req ApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	res, err := h.svc.Apply(c.Request.Context(), userID, c.Param("id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Revisions(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	res, err := h.svc.Revisions(c.Request.Context(), userID, c.Param("id"), limit)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Restore(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}

	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	res, err := h.svc.Restore(c.Request.Context(), userID, c.Param("id"), *req.Revision)
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
	case errors.Is(err, ErrTableNotFound):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, ErrRevisionNotFound):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, ErrRevisionConflict):
		response.Error(c, http.StatusConflict, 409, err.Error())
	case errors.Is(err, ErrOpsRejected):
		// 422：请求本身合法，是其中的变更内容没过校验
		response.Error(c, http.StatusUnprocessableEntity, 422, err.Error())
	case errors.Is(err, ErrNothingToApply):
		response.Error(c, http.StatusBadRequest, 400, err.Error())
	case errors.Is(err, ErrTitleRequired):
		response.Error(c, http.StatusBadRequest, 400, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, 500, "服务器内部错误")
	}
}
