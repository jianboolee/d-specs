package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Body{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Body{
		Code:    code,
		Message: message,
	})
}

func NotImplemented(c *gin.Context, message string) {
	Error(c, http.StatusNotImplemented, 501, message)
}
