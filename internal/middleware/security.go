package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders 为所有响应添加基础安全头，降低点击劫持、MIME 嗅探、
// referrer 泄露等风险。无论部署在内网、公网还是反代之后都作为兜底保护。
// 注：HSTS 仅在全站 HTTPS 时安全，故不在应用层强制下发，可在反代层按需开启。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-XSS-Protection", "1; mode=block")
		// 宽松 CSP：保留 inline script/style 以兼容前端构建产物，避免破坏功能。
		// 如需更严格策略，建议改为 nonce 方案并在反代层统一配置。
		c.Header("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")
		c.Next()
	}
}
