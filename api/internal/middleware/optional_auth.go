package middleware

import (
	"strings"

	"github.com/donktech/d-specs/internal/contextx"
	infraauth "github.com/donktech/d-specs/internal/infrastructure/auth"

	"github.com/gin-gonic/gin"
)

// OptionalAuth 若携带有效 Bearer Token 则写入用户 ID，否则匿名继续
func OptionalAuth(jwtManager *infraauth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.Next()
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Next()
			return
		}
		claims, err := jwtManager.Verify(parts[1])
		if err == nil && claims.UserID != "" {
			c.Request = c.Request.WithContext(contextx.WithUserID(c.Request.Context(), claims.UserID))
		}
		c.Next()
	}
}
