package share

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func sampleSpec() *PublicSpec {
	return &PublicSpec{
		Title:       "手机参数表",
		Description: "某型号的完整参数",
		Industry:    "数码",
		UpdatedAt:   time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Groups: []PublicGroup{
			{
				Name: "续航",
				Items: []PublicItem{
					{Name: "电池容量", Value: "5000mAh", ValueType: "number"},
					{Name: "充电功率", Value: "", ValueType: "number"},
				},
			},
		},
	}
}

func newTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "http://specs.example.com/s/abc", nil)
	return c
}

// 分享页的核心价值：不执行 JS 的爬虫也要能从 HTML 里直接读到正文。
// 因此正文必须出现在渲染结果里，且标题/描述进 <head>。
func TestRenderIncludesHeadAndBody(t *testing.T) {
	p := NewPageRenderer("/nonexistent-dist", "参数配置表", "https://specs.example.com")
	out := string(p.Render(newTestContext(), "abc", sampleSpec()))

	for _, want := range []string{
		`<title>手机参数表 - 参数配置表</title>`,
		`name="description" content="某型号的完整参数"`,
		`property="og:title" content="手机参数表 - 参数配置表"`,
		`property="og:type" content="article"`,
		`property="og:url" content="https://specs.example.com/s/abc"`,
		`rel="canonical" href="https://specs.example.com/s/abc"`,
		`application/ld+json`,
		`<h1>手机参数表</h1>`,
		`电池容量`,
		`5000mAh`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("渲染结果缺少 %q\n%s", want, out)
		}
	}
}

// 没有值时应显示占位文案，而不是渲染成空白单元格。
func TestRenderEmptyValuePlaceholder(t *testing.T) {
	out := string(NewPageRenderer("/none", "参数配置表", "").Render(newTestContext(), "abc", sampleSpec()))
	if !strings.Contains(out, "待补充") {
		t.Fatalf("空值应显示占位文案：%s", out)
	}
}

// 用户填的标题可能含 HTML，注入到 HTML 里必须转义，避免 XSS。
func TestRenderEscapesUserContent(t *testing.T) {
	spec := sampleSpec()
	spec.Title = `<script>alert(1)</script>`
	out := string(NewPageRenderer("/none", "参数配置表", "").Render(newTestContext(), "abc", spec))

	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Fatal("标题里的脚本标签被原样输出，存在 XSS")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("标题应被 HTML 转义：%s", out)
	}
}

// 关闭的分享必须返回 404 + noindex，否则过期页面会一直留在搜索索引里。
func TestRenderNotFoundIsNoindex(t *testing.T) {
	out := string(NewPageRenderer("/none", "参数配置表", "").RenderNotFound(newTestContext()))
	if !strings.Contains(out, `content="noindex, nofollow"`) {
		t.Fatalf("404 页应声明 noindex：%s", out)
	}
	if !strings.Contains(out, "无法打开这份参数表") {
		t.Fatalf("404 页应给出可读文案：%s", out)
	}
}

// 描述缺失时用参数名兜底，避免搜索结果里出现空描述。
func TestMetaDescriptionFallback(t *testing.T) {
	spec := sampleSpec()
	spec.Description = ""

	desc := metaDescription(spec, countItems(spec))
	if !strings.Contains(desc, "手机参数表") || !strings.Contains(desc, "电池容量") {
		t.Fatalf("兜底描述应包含标题与参数名：%q", desc)
	}
}

// 结构化数据必须是合法 JSON，否则搜索引擎无法解析。
func TestStructuredDataIsValidJSON(t *testing.T) {
	raw := structuredData(sampleSpec(), 2, "https://specs.example.com/s/abc", "参数配置表")

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("JSON-LD 不是合法 JSON: %v\n%s", err, raw)
	}
	if parsed["@type"] != "Article" {
		t.Fatalf("期望 Article 类型，实际 %v", parsed["@type"])
	}
}

// 用真实的前端构建产物验证注入：Vite 必须原样保留 <!--seo-head--> / <!--seo-body-->
// 锚点，否则 SSR 会静默失效（页面仍是空壳）。产物缺失时跳过，不阻塞单测。
func TestRenderAgainstRealBuild(t *testing.T) {
	dist := filepath.Join("..", "..", "..", "..", "web", "dist")
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		t.Skip("web/dist 不存在，跳过产物注入检查（先跑 npm run build）")
	}

	p := NewPageRenderer(dist, "参数配置表", "https://specs.21rv.com")
	out := string(p.Render(newTestContext(), "abc", sampleSpec()))

	if strings.Contains(out, headMarker) || strings.Contains(out, bodyMarker) {
		t.Fatal("注入锚点未被替换，index.html 可能缺少 <!--seo-head--> / <!--seo-body-->")
	}
	for _, want := range []string{
		`<title>手机参数表 - 参数配置表</title>`,
		`rel="canonical" href="https://specs.21rv.com/s/abc"`,
		`<h1>手机参数表</h1>`,
		`5000mAh`,
		// Vite 注入的模块脚本必须仍在，否则 Vue 无法接管页面
		`<script type="module"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("真实产物渲染结果缺少 %q", want)
		}
	}
}

// 站点地址未显式配置时，应回退到 X-Forwarded-* 请求头推导绝对地址。
func TestAbsURLFallsBackToForwardedHeaders(t *testing.T) {
	p := NewPageRenderer("/none", "参数配置表", "")

	c := newTestContext()
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Header.Set("X-Forwarded-Host", "specs.21rv.com")

	if got := p.absURL(c, "/s/abc"); got != "https://specs.21rv.com/s/abc" {
		t.Fatalf("按转发头推导的地址不对：%s", got)
	}
}

// 显式配置的 siteURL 优先，不依赖可被伪造的请求头。
func TestAbsURLPrefersConfiguredSiteURL(t *testing.T) {
	p := NewPageRenderer("/none", "参数配置表", "https://specs.21rv.com")

	c := newTestContext()
	c.Request.Header.Set("X-Forwarded-Host", "evil.example.com")

	if got := p.absURL(c, "/s/abc"); got != "https://specs.21rv.com/s/abc" {
		t.Fatalf("显式 siteURL 应优先：%s", got)
	}
}
