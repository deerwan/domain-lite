package static

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// frontendNotBuiltHTML 在前端未构建（dist 为空 / 缺少 index.html）时返回明确提示，
// 避免静默返回 404/301 让人摸不着头脑。
const frontendNotBuiltHTML = `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><title>domain-lite</title></head>
<body style="font-family:system-ui,sans-serif;padding:2rem;line-height:1.6">
<h1>前端未构建</h1>
<p>当前镜像未包含前端静态资源（<code>internal/static/dist</code> 为空或缺少 <code>index.html</code>）。</p>
<p>请使用 CI 完整构建的镜像，或本地执行 <code>pnpm build</code> 后将 <code>web/dist</code> 拷入 <code>internal/static/dist</code> 再编译。</p>
</body></html>`

// Handler 提供前端静态资源服务，并对未知前端路由做 SPA 回退。
// 以 /api 开头的请求不会被此处处理（交由 API 路由/404）。
func Handler() gin.HandlerFunc {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// dist 为空（如本地仅跑后端时）则跳过静态服务
		return notBuiltHandler()
	}
	if !frontendBuilt(sub) {
		return notBuiltHandler()
	}

	fileServer := http.FileServer(http.FS(sub))
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			c.Status(http.StatusNotFound)
			return
		}
		// 直接请求 /、/index.html，或未知前端路由：一律返回 index.html（SPA 入口）。
		// 直接读取字节返回，彻底绕开 http.FileServer 在嵌入式 dirFS 上把根目录当目录做的 301 跳转。
		if path == "/" || path == "/index.html" {
			serveIndex(c, sub)
			return
		}
		f, openErr := sub.Open(strings.TrimPrefix(path, "/"))
		if openErr != nil {
			serveIndex(c, sub)
			return
		}
		if info, statErr := f.Stat(); statErr == nil && info.IsDir() {
			f.Close()
			serveIndex(c, sub)
			return
		}
		f.Close()
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}

// serveIndex 读取并返回 index.html 字节（SPA 入口），避免任何目录 301 跳转。
func serveIndex(c *gin.Context, sub fs.FS) {
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

// notBuiltHandler 在前端缺失时返回明确提示页（/api 仍走 404）。
func notBuiltHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(frontendNotBuiltHTML))
	}
}

// frontendBuilt 检测 dist 是否包含 index.html（真实前端构建产物）。
func frontendBuilt(sub fs.FS) bool {
	f, err := sub.Open("index.html")
	if err != nil {
		return false
	}
	f.Close()
	return true
}
