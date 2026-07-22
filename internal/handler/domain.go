package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"domain-lite/internal/config"
	"domain-lite/internal/crypto"
	"domain-lite/internal/model"
	"domain-lite/internal/notify"
	"domain-lite/internal/provider"
	"domain-lite/internal/whois"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DomainHandler 管理域名集合（识别逻辑后续阶段补充）。
type DomainHandler struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewDomainHandler(cfg *config.Config, db *gorm.DB) *DomainHandler {
	return &DomainHandler{cfg: cfg, db: db}
}

type domainReq struct {
	Domain       string `json:"domain"`
	DnsAccountID uint   `json:"dns_account_id"`
	ZoneID       string `json:"zone_id"`
	Note         string `json:"note"`
}

// DiscoveredDomain 从 DNS 账户自动识别到的域名（区别于手动录入的 Domain 表）。
type DiscoveredDomain struct {
	Domain      string     `json:"domain"`
	ZoneID      string     `json:"zone_id"`
	AccountID   uint       `json:"account_id"`
	AccountName string     `json:"account_name"`
	AccountType string     `json:"account_type"`
	Registrar   string     `json:"registrar"`
	ExpireAt    *time.Time `json:"expire_at"`
	Status      string     `json:"status"`
	WhoisManual bool       `json:"whois_manual"`
}

// collectZones 并发拉取当前用户所有 DNS 账户下的域名(zone)，
// 返回域名列表与每个失败账户的报错。被 Discover 与 EnrichWHOIS 复用。
func (h *DomainHandler) collectZones(ctx context.Context, uid any) ([]DiscoveredDomain, []map[string]string) {
	var accounts []model.DnsAccount
	h.db.Where("user_id = ?", uid).Find(&accounts)

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		out  = make([]DiscoveredDomain, 0, len(accounts))
		errs = make([]map[string]string, 0)
	)
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	for _, acc := range accounts {
		wg.Add(1)
		go func(acc model.DnsAccount) {
			defer wg.Done()
			p, err := provider.New(h.cfg, &acc)
			if err != nil {
				mu.Lock()
				errs = append(errs, map[string]string{"account": acc.Name, "error": "构建账户失败: " + err.Error()})
				mu.Unlock()
				return
			}
			zones, err := p.ListZones(ctx)
			if err != nil {
				mu.Lock()
				errs = append(errs, map[string]string{"account": acc.Name, "error": err.Error()})
				mu.Unlock()
				return
			}
			disc := make([]DiscoveredDomain, 0, len(zones))
			for _, z := range zones {
				disc = append(disc, DiscoveredDomain{
					Domain:      z.Name,
					ZoneID:      z.ID,
					AccountID:   acc.ID,
					AccountName: acc.Name,
					AccountType: acc.Type,
				})
			}
			mu.Lock()
			out = append(out, disc...)
			mu.Unlock()
		}(acc)
	}
	wg.Wait()
	return out, errs
}

// attachCache 将已 enrich 的 WHOIS 缓存(注册商/到期日/状态) 附加到聚合域名列表上。
func (h *DomainHandler) attachCache(uid any, disc []DiscoveredDomain) {
	var cached []model.Domain
	h.db.Where("user_id = ?", uid).Find(&cached)
	cacheMap := make(map[string]model.Domain, len(cached))
	for _, d := range cached {
		cacheMap[d.Domain+"|"+strconv.Itoa(int(d.DnsAccountID))] = d
	}
	for i := range disc {
		if cd, ok := cacheMap[disc[i].Domain+"|"+strconv.Itoa(int(disc[i].AccountID))]; ok {
			disc[i].Registrar = cd.Registrar
			disc[i].ExpireAt = cd.ExpireAt
			disc[i].Status = cd.Status
			disc[i].WhoisManual = cd.WhoisManual
		}
	}
}

// Discover 实时聚合所有 DNS 账户的域名，并附带已缓存的 WHOIS(注册商/到期日/状态)。
func (h *DomainHandler) Discover(c *gin.Context) {
	uid, _ := c.Get("uid")
	disc, errs := h.collectZones(c.Request.Context(), uid)
	h.attachCache(uid, disc)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": disc, "errors": errs})
}

// EnrichWHOIS 对所有已识别域名查询 WHOIS，填充注册商/到期日/状态，并触发临期提醒。
func (h *DomainHandler) EnrichWHOIS(c *gin.Context) {
	uid, _ := c.Get("uid")
	total, success, failed, failedList := h.enrichAll(c.Request.Context(), uid)
	h.notifyExpiring(uid)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"total":      total,
			"success":    success,
			"failed":     failed,
			"failedList": failedList,
		},
	})
}

// enrichAll 对所有已识别域名并发查询 WHOIS 并落库，返回统计。与 HTTP 层解耦，供定时任务复用。
func (h *DomainHandler) enrichAll(ctx context.Context, uid any) (total, success, failed int, failedList []string) {
	disc, _ := h.collectZones(ctx, uid)

	type job struct {
		domain    string
		accountID uint
		zoneID    string
	}
	seen := make(map[string]bool)
	jobs := make([]job, 0)
	for _, d := range disc {
		key := d.Domain + "|" + strconv.Itoa(int(d.AccountID))
		if seen[key] {
			continue
		}
		seen[key] = true
		jobs = append(jobs, job{d.Domain, d.AccountID, d.ZoneID})
	}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	sem := make(chan struct{}, 5)
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			info, err := whois.Lookup(ctx, j.domain)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				failedList = append(failedList, j.domain+": "+err.Error())
				return
			}
			var d model.Domain
			// GORM 的 FirstOrCreate 内联 Where 条件仅用于查询，不会作为新建记录的字段值，
			// 必须显式把归属字段写到 struct 上，否则新建行 user_id/dns_account_id 会变成 0。
			if u, ok := uid.(uint); ok {
				d.UserID = u
			}
			d.DnsAccountID = j.accountID
			d.Domain = j.domain
			h.db.Where("user_id = ? AND domain = ? AND dns_account_id = ?", uid, j.domain, j.accountID).
				FirstOrCreate(&d)
			// 强策略：手动钉住的行整行跳过，WHOIS 结果一律不写（含 last_check）。
			if d.WhoisManual {
				success++
				return
			}
			// 空值守卫：WHOIS 可能查通但解析不出某字段，
			// 只写非空字段，避免把库里已有的正确值覆盖成空。
			upd := map[string]any{
				"zone_id":    j.zoneID,
				"last_check": time.Now(),
			}
			if info.Registrar != "" {
				upd["registrar"] = info.Registrar
			}
			if info.ExpireAt != nil {
				upd["expire_at"] = info.ExpireAt
			}
			if info.Status != "" {
				upd["status"] = info.Status
			}
			h.db.Model(&d).Updates(upd)
			success++
		}(j)
	}
	wg.Wait()
	total = len(jobs)
	return
}

// buildNotifyConfig 读取当前用户的通知配置：优先用数据库中的设置，未配置则回退到 env 默认值。
// 返回 nil 表示无需发送（未启用或无 Webhook 地址）。
func (h *DomainHandler) buildNotifyConfig(uid any) *notify.Config {
	var s model.NotifySetting
	if err := h.db.Where("user_id = ?", uid).First(&s).Error; err == nil {
		cfg := &notify.Config{
			Enabled:       s.Enabled,
			Type:          s.Type,
			ChatID:        s.ChatID,
			ThresholdDays: s.ThresholdDays,
		}
		if s.WebhookURLEnc != "" {
			if url, derr := crypto.Decrypt(s.WebhookURLEnc, h.cfg.JWTSecret); derr == nil {
				cfg.WebhookURL = url
			}
		}
		if cfg.Type == "" {
			cfg.Type = h.cfg.NotifyType
		}
		if cfg.ThresholdDays <= 0 {
			cfg.ThresholdDays = h.cfg.NotifyThreshold
		}
		return cfg
	}
	// 回退到 env 默认值
	return &notify.Config{
		Enabled:       h.cfg.NotifyEnabled,
		Type:          h.cfg.NotifyType,
		WebhookURL:    h.cfg.NotifyWebhookURL,
		ChatID:        h.cfg.NotifyChatID,
		ThresholdDays: h.cfg.NotifyThreshold,
	}
}

// getThreshold 返回当前用户的临期提醒阈值（天），优先数据库设置，回退 env。
func (h *DomainHandler) getThreshold(uid any) int {
	var s model.NotifySetting
	if err := h.db.Where("user_id = ?", uid).First(&s).Error; err == nil && s.ThresholdDays > 0 {
		return s.ThresholdDays
	}
	if h.cfg.NotifyThreshold > 0 {
		return h.cfg.NotifyThreshold
	}
	return 30
}

// currentSyncIntervalMin 返回自动同步间隔（分钟），优先数据库设置，回退 env，兜底 720。
func (h *DomainHandler) currentSyncIntervalMin() int {
	var s model.NotifySetting
	if err := h.db.Order("id").First(&s).Error; err == nil && s.SyncIntervalMin > 0 {
		return s.SyncIntervalMin
	}
	if h.cfg.SyncIntervalMin > 0 {
		return h.cfg.SyncIntervalMin
	}
	return 720
}

// notifyExpiring 找出剩余天数 <= 阈值且未近期提醒过的域名，发送 Webhook 提醒并去重。
func (h *DomainHandler) notifyExpiring(uid any) {
	cfg := h.buildNotifyConfig(uid)
	if cfg == nil || !cfg.Enabled || cfg.WebhookURL == "" {
		return
	}
	if cfg.ThresholdDays <= 0 {
		cfg.ThresholdDays = 30
	}
	repeat := 7 * 24 * time.Hour // 同一域名最多每 7 天提醒一次

	var domains []model.Domain
	h.db.Where("user_id = ? AND expire_at IS NOT NULL", uid).Find(&domains)
	now := time.Now()
	var lines []string
	notified := make([]uint, 0, len(domains))
	for _, d := range domains {
		days := int(d.ExpireAt.Sub(now).Hours() / 24)
		if days > cfg.ThresholdDays {
			continue
		}
		if d.LastNotifiedAt != nil && now.Sub(*d.LastNotifiedAt) < repeat {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s 将于 %s 到期（剩 %d 天，注册商：%s）",
			d.Domain, d.ExpireAt.Format("2006-01-02"), days, d.Registrar))
		notified = append(notified, d.ID)
	}
	if len(lines) == 0 {
		return
	}
	if err := notify.Send(*cfg, "🔔 域名临期提醒", lines); err != nil {
		log.Printf("[notify] send failed: %v", err)
		return
	}
	// 标记已通知，避免重复轰炸
	for _, id := range notified {
		h.db.Model(&model.Domain{}).Where("id = ?", id).Update("last_notified_at", now)
	}
}

// ScheduleSync 后台定时同步 WHOIS 并发送临期提醒。启动时先跑一次，之后按周期循环。
// 同步间隔从数据库/环境变量动态读取，修改设置后下一次等待自动生效。
func (h *DomainHandler) ScheduleSync(ctx context.Context) {
	log.Printf("[sync] scheduler started")
	h.runSyncForAll(ctx)
	for {
		interval := h.currentSyncIntervalMin()
		select {
		case <-ctx.Done():
			log.Printf("[sync] scheduler stopped")
			return
		case <-time.After(time.Duration(interval) * time.Minute):
			h.runSyncForAll(ctx)
		}
	}
}

// runSyncForAll 对所有用户执行一次 WHOIS 同步与临期提醒（单用户自用场景通常只有一个用户）。
func (h *DomainHandler) runSyncForAll(ctx context.Context) {
	var users []model.User
	h.db.Find(&users)
	for _, u := range users {
		syncCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		total, success, failed, _ := h.enrichAll(syncCtx, u.ID)
		cancel()
		log.Printf("[sync] user=%d total=%d success=%d failed=%d", u.ID, total, success, failed)
		h.notifyExpiring(u.ID)
	}
}

// Stats 返回仪表盘所需的聚合统计：域名总数、临期/已过期数量、按账户分布、临期清单。
func (h *DomainHandler) Stats(c *gin.Context) {
	uid, _ := c.Get("uid")
	disc, errs := h.collectZones(c.Request.Context(), uid)
	h.attachCache(uid, disc)

	threshold := h.getThreshold(uid)
	now := time.Now()
	var total, expiringSoon, expired int
	byAccCount := make(map[uint]int)
	nameMap := make(map[uint]string)
	expiringList := make([]gin.H, 0)
	for _, d := range disc {
		total++
		byAccCount[d.AccountID]++
		nameMap[d.AccountID] = d.AccountName
		if d.ExpireAt == nil {
			continue
		}
		days := int(d.ExpireAt.Sub(now).Hours() / 24)
		if days < 0 {
			expired++
		} else if days <= threshold {
			expiringSoon++
			expiringList = append(expiringList, gin.H{
				"domain":       d.Domain,
				"expire_at":    d.ExpireAt,
				"days_left":    days,
				"registrar":    d.Registrar,
				"status":       d.Status,
				"account_name": d.AccountName,
			})
		}
	}
	sort.Slice(expiringList, func(i, j int) bool {
		return expiringList[i]["days_left"].(int) < expiringList[j]["days_left"].(int)
	})

	var accounts int64
	h.db.Model(&model.DnsAccount{}).Where("user_id = ?", uid).Count(&accounts)

	byAccount := make([]gin.H, 0, len(byAccCount))
	for id, cnt := range byAccCount {
		byAccount = append(byAccount, gin.H{
			"account_id":   id,
			"account_name": nameMap[id],
			"count":        cnt,
		})
	}
	sort.Slice(byAccount, func(i, j int) bool {
		return byAccount[i]["account_id"].(uint) < byAccount[j]["account_id"].(uint)
	})

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"total_domains":  total,
		"expiring_soon":  expiringSoon,
		"expired":        expired,
		"accounts":       accounts,
		"threshold_days": threshold,
		"by_account":     byAccount,
		"expiring_list":  expiringList,
		"errors":         errs,
	}})
}

// List 返回当前用户的域名集合（手动录入）。
func (h *DomainHandler) List(c *gin.Context) {
	uid, _ := c.Get("uid")
	var domains []model.Domain
	h.db.Where("user_id = ?", uid).Find(&domains)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": domains})
}

// Create 新增域名。
func (h *DomainHandler) Create(c *gin.Context) {
	uid, _ := c.Get("uid")
	var req domainReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	d := model.Domain{
		UserID:       uid.(uint),
		Domain:       req.Domain,
		DnsAccountID: req.DnsAccountID,
		ZoneID:       req.ZoneID,
		Note:         req.Note,
	}
	h.db.Create(&d)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"id": d.ID}})
}

// Delete 删除域名。
func (h *DomainHandler) Delete(c *gin.Context) {
	uid, _ := c.Get("uid")
	res := h.db.Where("user_id = ? AND id = ?", uid, c.Param("id")).Delete(&model.Domain{})
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// whoisManualReq 手动设置/清除 WHOIS 信息的请求体，按「域名 + DNS 账户」定位缓存行。
type whoisManualReq struct {
	Domain       string     `json:"domain"`
	DnsAccountID uint       `json:"dns_account_id"`
	Registrar    string     `json:"registrar"`
	ExpireAt     *time.Time `json:"expire_at"`
	Status       string     `json:"status"`
}

// SetWhois 手动设置某域名的注册商/到期日/状态，并钉住(whois_manual=true)，
// 之后自动同步不再覆盖，直到用户「恢复自动」。用于 WHOIS 识别不到信息的域名兜底。
func (h *DomainHandler) SetWhois(c *gin.Context) {
	uid, _ := c.Get("uid")
	var req whoisManualReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	var d model.Domain
	if u, ok := uid.(uint); ok {
		d.UserID = u
	}
	d.Domain = req.Domain
	d.DnsAccountID = req.DnsAccountID
	h.db.Where("user_id = ? AND domain = ? AND dns_account_id = ?", uid, req.Domain, req.DnsAccountID).
		FirstOrCreate(&d)
	h.db.Model(&d).Updates(map[string]any{
		"registrar":    req.Registrar,
		"expire_at":    req.ExpireAt,
		"status":       req.Status,
		"whois_manual": true,
		"last_check":   time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// ClearWhois 取消手动钉住(whois_manual=false)，让下一次自动同步重新以真实 WHOIS 为准。
func (h *DomainHandler) ClearWhois(c *gin.Context) {
	uid, _ := c.Get("uid")
	var req whoisManualReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	res := h.db.Model(&model.Domain{}).
		Where("user_id = ? AND domain = ? AND dns_account_id = ?", uid, req.Domain, req.DnsAccountID).
		Update("whois_manual", false)
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}
