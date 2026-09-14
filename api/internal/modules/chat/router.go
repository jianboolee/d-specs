package chat

import (
	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/middleware"
	"github.com/jianboolee/d-specs/internal/modules/analysis"
	"github.com/jianboolee/d-specs/internal/modules/share"
	"github.com/jianboolee/d-specs/internal/modules/spec"

	"github.com/gin-gonic/gin"
)

// Register 注册对话相关路由。全部需要登录。
//
// specSvc 由调用方传入而不是在这里另建一个：AI 变更与手工变更必须共用
// 同一个 spec.Service，否则两边的校验与版本语义会各走一套。
// analysisSvc 同理：对话里的「完善参数表」直接复用分析能力。
// shareSvc 也同理：对话里的「开启/关闭分享」要与手工分享共用同一套边界。
func Register(rg *gin.RouterGroup, d *deps.Deps, specSvc *spec.Service, analysisSvc *analysis.Service, shareSvc *share.Service) {
	repo := NewRepository(d.DB)
	svc := NewService(repo, specSvc, analysisSvc, shareSvc, d.LLM, d.Config)
	h := NewHandler(svc)

	authed := rg.Group("")
	authed.Use(middleware.AuthRequired(d.JWT))

	// SSE：流式返回整轮对话
	authed.POST("/tables/:id/chat", h.Stream)
	// SSE：「完善参数表」分析，形态上也是一轮对话
	authed.POST("/tables/:id/analyze", h.Analyze)

	authed.GET("/tables/:id/sessions", h.Sessions)
	authed.GET("/sessions/:sid/messages", h.Messages)
}
