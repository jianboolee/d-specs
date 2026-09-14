package share

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jianboolee/d-specs/internal/config"
	"github.com/jianboolee/d-specs/internal/deps"
	"github.com/jianboolee/d-specs/internal/modules/spec"

	"github.com/gin-gonic/gin"
)

// gin 的路由通配符冲突（如 /s/:id 与 /sitemap.xml 同层）只在注册时 panic，
// 编译期发现不了。这个测试确保路由装配不炸，并覆盖分享相关路径。
func TestRegisterRoutesDoNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")

	d := &deps.Deps{
		Config: &config.Config{
			SiteName:   "参数配置表",
			WebDistDir: "/nonexistent",
		},
	}
	specSvc := spec.NewService(spec.NewRepository(nil))

	// 出错会以 panic 形式暴露；返回值供 chat 模块复用（对话里开关分享）
	svc := Register(engine, v1, d, specSvc)
	if svc == nil {
		t.Fatal("Register 应返回 share.Service 供 chat 复用")
	}

	routes := map[string]bool{}
	for _, r := range engine.Routes() {
		routes[r.Method+" "+r.Path] = true
	}

	for _, want := range []string{
		"GET /s/:id",
		"GET /sitemap.xml",
		"GET /robots.txt",
		"GET /api/v1/public/specs",
		"GET /api/v1/public/specs/:id",
	} {
		if !routes[want] {
			t.Errorf("缺少路由 %s，实际有：%v", want, routes)
		}
	}
}

// robots.txt 不依赖数据库，可直接端到端验证。
func TestRobotsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := NewHandler(nil, NewPageRenderer("/none", "参数配置表", "https://specs.21rv.com"))
	engine.GET("/robots.txt", h.Robots)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	req.Host = "specs.21rv.com"
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("robots.txt 应返回 200，实际 %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Allow: /s/") {
		t.Fatalf("robots 应放行分享页：%s", body)
	}
	if !strings.Contains(body, "Sitemap: https://specs.21rv.com/sitemap.xml") {
		t.Fatalf("robots 应指向 sitemap：%s", body)
	}
	if !strings.Contains(body, "Disallow: /api/") {
		t.Fatalf("robots 应屏蔽接口：%s", body)
	}
}
