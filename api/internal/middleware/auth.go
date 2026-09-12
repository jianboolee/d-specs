package middleware

import (
	"net/http"
	"strings"

	"github.com/jianboolee/d-specs/internal/contextx"
	infraauth "github.com/jianboolee/d-specs/internal/infrastructure/auth"
	"github.com/jianboolee/d-specs/internal/response"

	"github.com/gin-gonic/gin"
)

func AuthRequired(jwtManager *infraauth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			response.Error(c, http.StatusUnauthorized, 401, "请先登录")
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, 401, "登录凭证格式不正确")
			c.Abort()
			return
		}

		claims, err := jwtManager.Verify(parts[1])
		if err != nil {
			response.Error(c, http.StatusUnauthorized, 401, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		c.Request = c.Request.WithContext(contextx.WithUserID(c.Request.Context(), claims.UserID))
		c.Next()
	}
}
