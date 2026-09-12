package auth

import (
	"errors"
	"net/http"

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

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	res, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Created(c, res)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	res, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *Handler) Me(c *gin.Context) {
	userID, err := contextx.RequireUserID(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusUnauthorized, 401, "未登录")
		return
	}

	profile, err := h.svc.Me(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, profile)
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrEmailTaken):
		response.Error(c, http.StatusConflict, 409, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, 401, err.Error())
	case errors.Is(err, ErrUserNotFound):
		response.Error(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, ErrWeakPassword):
		response.Error(c, http.StatusBadRequest, 400, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, 500, "服务器内部错误")
	}
}
