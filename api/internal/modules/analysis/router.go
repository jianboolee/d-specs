package analysis

import (
	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/middleware"
	"github.com/jianboolee/d-specs/internal/modules/spec"

	"github.com/gin-gonic/gin"
)

// Register 注册 AI 分析路由。需要登录。
//
// specSvc 由调用方传入而不是在这里另建：分析要读当前表内容，
// 且标签落库必须与手工/对话变更共用同一套写路径。
func Register(rg *gin.RouterGroup, d *deps.Deps, specSvc *spec.Service) {
	svc := NewService(specSvc, d.LLM, d.Config)
	h := NewHandler(svc)

	authed := rg.Group("")
	authed.Use(middleware.AuthRequired(d.JWT))

	authed.POST("/tables/:id/analyze", h.Analyze)
}
