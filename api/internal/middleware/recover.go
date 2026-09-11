package middleware

import (
	"log/slog"
	"net/http"

	"github.com/donktech/d-specs/internal/response"

	"github.com/gin-gonic/gin"
)

func Recover(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered", "error", r, "request_id", c.GetString(RequestIDKey))
				response.Error(c, http.StatusInternalServerError, 500, "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
