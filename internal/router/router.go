package router

import (
	"net/http"

	"domain-lite/internal/config"
	"domain-lite/internal/handler"
	"domain-lite/internal/middleware"
	"domain-lite/internal/static"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// New 构建 Gin 引擎并注册路由。
func New(cfg *config.Config, db *gorm.DB) *gin.Engine {
	r := gin.Default()
	r.Use(middleware.SecurityHeaders())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	authH := handler.NewAuthHandler(cfg, db)
	api.POST("/auth/login", authH.Login)
	api.POST("/auth/register", authH.Register)

	authed := api.Group("")
	authed.Use(middleware.JWTAuth(cfg))
	{
		authed.GET("/auth/me", authH.Me)

		dnsAccH := handler.NewDnsAccountHandler(cfg, db)
		authed.GET("/dns-accounts", dnsAccH.List)
		authed.POST("/dns-accounts", dnsAccH.Create)
		authed.DELETE("/dns-accounts/:id", dnsAccH.Delete)

		notifyH := handler.NewNotifySettingHandler(cfg, db)
		authed.GET("/notify-settings", notifyH.Get)
		authed.PUT("/notify-settings", notifyH.Update)
		authed.POST("/notify-settings/test", notifyH.Test)

		// 通过 DNS 账户操作服务商侧的域名(zones)与解析记录(records)
		dnsH := handler.NewDnsHandler(cfg, db)
		authed.GET("/dns-accounts/:id/zones", dnsH.ListZones)
		authed.GET("/dns-accounts/:id/zones/:zone/records", dnsH.ListRecords)
		authed.POST("/dns-accounts/:id/zones/:zone/records", dnsH.CreateRecord)
		authed.PUT("/dns-accounts/:id/zones/:zone/records/:record", dnsH.UpdateRecord)
		authed.DELETE("/dns-accounts/:id/zones/:zone/records/:record", dnsH.DeleteRecord)
		authed.GET("/dns-accounts/:id/zones/:zone/records/logs", dnsH.ListLogs)
		// 解析记录变更审计日志（全账户）
		authed.GET("/record-logs", dnsH.ListLogs)

		domainH := handler.NewDomainHandler(cfg, db)
		authed.GET("/domains", domainH.List)
		authed.POST("/domains", domainH.Create)
		authed.DELETE("/domains/:id", domainH.Delete)
		// 聚合所有 DNS 账户已识别到的域名（用于「域名列表」）
		authed.GET("/domains/discovered", domainH.Discover)
		// 仪表盘统计（域名总数 / 临期 / 已过期 / 按账户分布）
		authed.GET("/stats", domainH.Stats)
		// 对所有已识别域名查询 WHOIS，填充注册商/到期日/状态
		authed.POST("/domains/enrich-whois", domainH.EnrichWHOIS)
		// 手动设置/清除某域名的 WHOIS 信息（识别不到时兜底，钉住后不被自动同步覆盖）
		authed.PUT("/domains/whois", domainH.SetWhois)
		authed.DELETE("/domains/whois", domainH.ClearWhois)
	}

	// 前端静态资源（SPA）。放在最后，避免覆盖 API 路由。
	r.NoRoute(static.Handler())
	return r
}
