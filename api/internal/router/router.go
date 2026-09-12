package router

import (
	"net/http"

	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/modules/auth"
	"github.com/jianboolee/d-specs/internal/modules/chat"
	"github.com/jianboolee/d-specs/internal/modules/share"
	"github.com/jianboolee/d-specs/internal/modules/spec"
	"github.com/jianboolee/d-specs/internal/response"

	"github.com/gin-gonic/gin"
)

func Setup(engine *gin.Engine, d *deps.Deps) {
	engine.GET("/health", func(c *gin.Context) {
		response.OK(c, gin.H{
			"status": "ok",
			"llm":    d.LLM != nil,
		})
	})

	engine.GET("/health/ready", func(c *gin.Context) {
		ready := true
		checks := gin.H{}

		if d.DB != nil {
			sqlDB, err := d.DB.DB()
			if err != nil {
				ready = false
				checks["postgres"] = err.Error()
			} else if err := sqlDB.PingContext(c.Request.Context()); err != nil {
				ready = false
				checks["postgres"] = err.Error()
			} else {
				checks["postgres"] = "ok"
			}
		} else {
			checks["postgres"] = "unavailable"
			ready = false
		}

		if d.LLM != nil {
			checks["llm"] = d.LLM.Name() + ":" + d.LLM.Model()
		} else {
			checks["llm"] = "unavailable"
		}

		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, gin.H{
			"ready":  ready,
			"checks": checks,
		})
	})

	v1 := engine.Group("/api/v1")
	auth.Register(v1, d)

	// spec 的 Service 要传给 chat 与 share：
	// AI 变更必须与手工变更共用同一套执行器，分享要复用同一套归属校验
	specSvc := spec.Register(v1, d)
	chat.Register(v1, d, specSvc)
	// engine 也要传进去：分享页 /s/:id 与 sitemap / robots 挂在根路径，
	// 而不是 /api/v1 下
	share.Register(engine, v1, d, specSvc)
}
