package share

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 前端 index.html 里的注入锚点，见 web/index.html。
//
// 分享页走服务端渲染：Go 把 SEO 用的 <head> 标签与可被爬虫直接读到的正文
// 塞进入口 HTML 再返回。Vue 挂载时会清空 #app 并接管渲染，用户看到的是
// 完整交互页面；不执行 JS 的爬虫（百度、微信/飞书链接预览）读到的则是这里
// 渲染出来的静态内容。
const (
	headMarker = "<!--seo-head-->"
	bodyMarker = "<!--seo-body-->"
	// seedElementID 前端据此读取服务端注入的初始数据（见 web/src/views/ShareView.vue）
	seedElementID = "__SHARE_SPEC__"
)

// titleTag 用于把入口 HTML 里静态的 <title> 替换成当前分享页标题。
var titleTag = regexp.MustCompile(`<title>[^<]*</title>`)

// PageRenderer 负责把 PublicSpec 渲染进前端入口 HTML。
type PageRenderer struct {
	distDir  string
	siteName string
	// siteURL 为空的场景（本地开发）按请求头推导绝对地址
	siteURL string
}

func NewPageRenderer(distDir, siteName, siteURL string) *PageRenderer {
	return &PageRenderer{distDir: distDir, siteName: siteName, siteURL: siteURL}
}

// Render 渲染一份可被收录的分享页。
func (p *PageRenderer) Render(c *gin.Context, id string, spec *PublicSpec) []byte {
	total := countItems(spec)
	canonical := p.absURL(c, "/s/"+id)

	doc := p.shell()
	doc = strings.Replace(doc, headMarker, p.headBlock(c, spec, total, canonical), 1)
	doc = strings.Replace(doc, bodyMarker, p.bodyBlock(spec, total), 1)
	doc = titleTag.ReplaceAllString(doc, "<title>"+html.EscapeString(p.pageTitle(spec))+"</title>")
	return []byte(doc)
}

// RenderNotFound 分享不存在或已关闭时返回的页面。
//
// 必须带 404 状态码并声明 noindex：否则搜索引擎会把「已关闭的分享」当成
// 有效页面继续收录，过期内容会一直留在索引里。
func (p *PageRenderer) RenderNotFound(c *gin.Context) []byte {
	doc := p.shell()
	head := `<meta name="robots" content="noindex, nofollow">` + "\n    " + headMarker
	doc = strings.Replace(doc, headMarker, head, 1)
	body := `<div class="share-page"><main class="share-main"><div class="state error">` +
		`<h1>无法打开这份参数表</h1>` +
		`<p>可能是创建者已关闭分享，或者链接不完整。</p></div></main></div>`
	doc = strings.Replace(doc, bodyMarker, body, 1)
	doc = titleTag.ReplaceAllString(doc, "<title>页面不存在</title>")
	return []byte(doc)
}

// ==================== 组装 ====================

func (p *PageRenderer) shell() string {
	raw, err := os.ReadFile(filepath.Join(p.distDir, "index.html"))
	if err != nil || len(raw) == 0 {
		// 产物读不到（本地开发指向了不存在的目录）时退化成自包含页面，
		// 保证分享内容本身仍可被读取，而不是直接 500
		return fallbackShell
	}
	return string(raw)
}

func (p *PageRenderer) pageTitle(spec *PublicSpec) string {
	title := strings.TrimSpace(spec.Title)
	if title == "" {
		title = p.siteName
	}
	if p.siteName == "" || title == p.siteName {
		return title
	}
	return title + " - " + p.siteName
}

func (p *PageRenderer) headBlock(c *gin.Context, spec *PublicSpec, total int, canonical string) string {
	title := p.pageTitle(spec)
	desc := metaDescription(spec, total)
	image := p.absURL(c, "/logo.png")

	var b strings.Builder

	writeMeta := func(property, content string, isName bool) {
		attr := "property"
		if isName {
			attr = "name"
		}
		if content == "" {
			return
		}
		b.WriteString(`<meta ` + attr + `="` + property + `" content="` + html.EscapeString(content) + `">`)
		b.WriteString("\n    ")
	}

	writeMeta("description", desc, true)
	writeMeta("og:type", "article", false)
	writeMeta("og:site_name", p.siteName, false)
	writeMeta("og:title", title, false)
	writeMeta("og:description", desc, false)
	writeMeta("og:url", canonical, false)
	writeMeta("og:image", image, false)
	writeMeta("twitter:card", "summary_large_image", true)
	writeMeta("twitter:title", title, true)
	writeMeta("twitter:description", desc, true)
	writeMeta("twitter:image", image, true)

	b.WriteString(`<link rel="canonical" href="` + html.EscapeString(canonical) + `">`)
	b.WriteString("\n    ")

	if ld := structuredData(spec, total, canonical, p.siteName); ld != "" {
		b.WriteString(`<script type="application/ld+json">` + ld + `</script>`)
		b.WriteString("\n    ")
	}

	// 把同一份数据交给前端作为初始状态：Vue 接管时无需再请求一次接口，
	// 也不会出现「完整内容 → 加载中 → 完整内容」的闪烁。
	// 放在 head 而不是 #app 内，避免被 Vue 挂载时清空。
	if seed := publicSpecSeed(spec); seed != "" {
		b.WriteString(`<script type="application/json" id="` + seedElementID + `">` + seed + `</script>`)
		b.WriteString("\n    ")
	}

	return b.String()
}

func (p *PageRenderer) bodyBlock(spec *PublicSpec, total int) string {
	var b strings.Builder

	// 外层 .share-page 是样式的命名空间根，必须与 ShareView.vue 的结构一致，
	// 否则注入的静态正文匹配不到 share.css 里的规则
	b.WriteString(`<div class="share-page">`)
	b.WriteString(`<main class="share-main">`)
	b.WriteString(`<div class="spec-head">`)
	b.WriteString(`<h1>` + html.EscapeString(spec.Title) + `</h1>`)
	if desc := strings.TrimSpace(spec.Description); desc != "" {
		b.WriteString(`<p class="spec-desc">` + html.EscapeString(desc) + `</p>`)
	}

	b.WriteString(`<div class="spec-meta">`)
	if industry := strings.TrimSpace(spec.Industry); industry != "" {
		b.WriteString(`<span class="tag">` + html.EscapeString(industry) + `</span>`)
	}
	// 产品类型标签（AI 分析写入），排在行业之后
	for _, tag := range spec.Tags {
		if t := strings.TrimSpace(tag); t != "" {
			b.WriteString(`<span class="tag">` + html.EscapeString(t) + `</span>`)
		}
	}
	b.WriteString(`<span>` + strconv.Itoa(total) + ` 个参数</span>`)
	if s := formatDate(spec.UpdatedAt); s != "" {
		b.WriteString(`<span class="dot">·</span><span>更新于 ` + s + `</span>`)
	}
	b.WriteString(`</div></div>`)

	for _, g := range spec.Groups {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			name = "参数"
		}
		b.WriteString(`<section class="group"><h2>` + html.EscapeString(name) + `</h2>`)
		b.WriteString(`<dl class="spec-list">`)
		for _, it := range g.Items {
			value := strings.TrimSpace(it.Value)
			empty := ""
			if value == "" {
				value = "待补充"
				empty = " empty"
			}
			b.WriteString(`<div class="spec-row"><dt>` + html.EscapeString(it.Name) + `</dt>`)
			b.WriteString(`<dd><span class="spec-value` + empty + `">` + html.EscapeString(value) + `</span></dd></div>`)
		}
		b.WriteString(`</dl></section>`)
	}

	b.WriteString(`</main>`)
	// 页脚与 ShareView.vue 保持一致，避免 SSR 与接管后视觉跳动
	b.WriteString(`<footer class="share-foot">由小野驴科技提供技术支持</footer>`)
	b.WriteString(`</div>`)
	return b.String()
}

// ==================== 辅助 ====================

type jsonLDPublisher struct {
	Type string `json:"@type"`
	Name string `json:"name"`
}

type jsonLDArticle struct {
	Context      string          `json:"@context"`
	Type         string          `json:"@type"`
	Headline     string          `json:"headline"`
	Description  string          `json:"description,omitempty"`
	URL          string          `json:"url"`
	DateModified string          `json:"dateModified,omitempty"`
	InLanguage   string          `json:"inLanguage"`
	Publisher    jsonLDPublisher `json:"publisher"`
}

func structuredData(spec *PublicSpec, total int, canonical, siteName string) string {
	payload := jsonLDArticle{
		Context:     "https://schema.org",
		Type:        "Article",
		Headline:    spec.Title,
		Description: metaDescription(spec, total),
		URL:         canonical,
		InLanguage:  "zh-CN",
		Publisher: jsonLDPublisher{
			Type: "Organization",
			Name: siteName,
		},
	}
	if !spec.UpdatedAt.IsZero() {
		payload.DateModified = spec.UpdatedAt.Format(time.RFC3339)
	}

	// json.Marshal 默认把 <、>、& 转义成 \u003c 等，可安全内联进 <script>
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

// publicSpecSeed 把 PublicSpec 序列化成可安全内联的 JSON。
//
// json.Marshal 会把 <、>、& 转义为 \u003c 等，因此用户填写的标题/描述里
// 即使含 </script> 也无法逃逸出 script 标签。
func publicSpecSeed(spec *PublicSpec) string {
	raw, err := json.Marshal(spec)
	if err != nil {
		return ""
	}
	return string(raw)
}

// metaDescription 优先用表描述；没有描述时按参数名兜底生成一段
// —— 空描述在搜索结果里既没吸引力，也不利于关键词匹配。
func metaDescription(spec *PublicSpec, total int) string {
	if desc := strings.TrimSpace(spec.Description); desc != "" {
		return truncateRunes(collapseSpace(desc), 120)
	}

	names := make([]string, 0, 6)
	for _, g := range spec.Groups {
		for _, it := range g.Items {
			if name := strings.TrimSpace(it.Name); name != "" {
				names = append(names, name)
			}
			if len(names) >= 6 {
				break
			}
		}
		if len(names) >= 6 {
			break
		}
	}

	desc := spec.Title + "：共 " + strconv.Itoa(total) + " 项参数"
	if len(names) > 0 {
		desc += "，包含 " + strings.Join(names, "、")
	}
	return truncateRunes(desc, 120)
}

func countItems(spec *PublicSpec) int {
	total := 0
	for _, g := range spec.Groups {
		total += len(g.Items)
	}
	return total
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(time.Local).Format("2006年1月2日")
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// absURL 拼出绝对地址。siteURL 优先（生产显式配置，canonical 稳定可信）；
// 未配置时按 nginx 传入的 X-Forwarded-Proto / Host 推导。
func (p *PageRenderer) absURL(c *gin.Context, path string) string {
	base := p.siteURL
	if base == "" {
		base = requestOrigin(c)
	}
	if base == "" {
		return path
	}
	return base + path
}

func requestOrigin(c *gin.Context) string {
	proto := c.GetHeader("X-Forwarded-Proto")
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		return ""
	}
	return proto + "://" + host
}

// fallbackShell 读不到前端产物时的兜底页面，保留注入锚点，保证流程一致。
const fallbackShell = `<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    ` + headMarker + `
    <title>参数配置表</title>
  </head>
  <body>
    <div id="app">` + bodyMarker + `</div>
  </body>
</html>`
