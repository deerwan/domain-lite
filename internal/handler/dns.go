package handler

import (
	"net/http"
	"strconv"

	"domain-lite/internal/config"
	"domain-lite/internal/model"
	"domain-lite/internal/provider"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DnsHandler 通过 DNS 账户操作服务商侧的域名(zones)与解析记录(records)。
type DnsHandler struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewDnsHandler(cfg *config.Config, db *gorm.DB) *DnsHandler {
	return &DnsHandler{cfg: cfg, db: db}
}

// providerFor 取出归属当前用户的账户并构造 provider。
func (h *DnsHandler) providerFor(c *gin.Context, accountID string) (provider.DnsProvider, bool) {
	uid, _ := c.Get("uid")
	aid, err := strconv.ParseUint(accountID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid account id"})
		return nil, false
	}
	var acc model.DnsAccount
	if err := h.db.Where("user_id = ? AND id = ?", uid, uint(aid)).First(&acc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "account not found"})
		return nil, false
	}
	p, err := provider.New(h.cfg, &acc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "build provider: " + err.Error()})
		return nil, false
	}
	return p, true
}

// ListZones GET /dns-accounts/:id/zones
func (h *DnsHandler) ListZones(c *gin.Context) {
	p, ok := h.providerFor(c, c.Param("id"))
	if !ok {
		return
	}
	zones, err := p.ListZones(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": zones})
}

// ListRecords GET /dns-accounts/:id/zones/:zone/records
func (h *DnsHandler) ListRecords(c *gin.Context) {
	p, ok := h.providerFor(c, c.Param("id"))
	if !ok {
		return
	}
	recs, err := p.ListRecords(c.Request.Context(), c.Param("zone"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": recs})
}

type recordReq struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority"`
	Proxied  *bool  `json:"proxied"`
}

// CreateRecord POST /dns-accounts/:id/zones/:zone/records
func (h *DnsHandler) CreateRecord(c *gin.Context) {
	p, ok := h.providerFor(c, c.Param("id"))
	if !ok {
		return
	}
	var req recordReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Type == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	r := provider.Record{Name: req.Name, Type: req.Type, Content: req.Content, TTL: req.TTL, Priority: req.Priority, Proxied: req.Proxied}
	id, err := p.CreateRecord(c.Request.Context(), c.Param("zone"), r)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
		return
	}
	h.logChange(c, c.Param("id"), c.Param("zone"), "create", req.Type, req.Name, "", req.Content)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"id": id}})
}

// UpdateRecord PUT /dns-accounts/:id/zones/:zone/records/:record
func (h *DnsHandler) UpdateRecord(c *gin.Context) {
	p, ok := h.providerFor(c, c.Param("id"))
	if !ok {
		return
	}
	var req recordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	before := h.recordContent(p, c, c.Param("zone"), c.Param("record"))
	r := provider.Record{Name: req.Name, Type: req.Type, Content: req.Content, TTL: req.TTL, Priority: req.Priority, Proxied: req.Proxied}
	if err := p.UpdateRecord(c.Request.Context(), c.Param("zone"), c.Param("record"), r); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
		return
	}
	h.logChange(c, c.Param("id"), c.Param("zone"), "update", req.Type, req.Name, before, req.Content)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// DeleteRecord DELETE /dns-accounts/:id/zones/:zone/records/:record
func (h *DnsHandler) DeleteRecord(c *gin.Context) {
	p, ok := h.providerFor(c, c.Param("id"))
	if !ok {
		return
	}
	before := h.recordContent(p, c, c.Param("zone"), c.Param("record"))
	if err := p.DeleteRecord(c.Request.Context(), c.Param("zone"), c.Param("record")); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
		return
	}
	h.logChange(c, c.Param("id"), c.Param("zone"), "delete", "", "", before, "")
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// recordContent 在变更前读取某条记录的当前内容，用于审计日志的「变更前」。
func (h *DnsHandler) recordContent(p provider.DnsProvider, c *gin.Context, zone, recordID string) string {
	recs, err := p.ListRecords(c.Request.Context(), zone)
	if err != nil {
		return ""
	}
	for _, r := range recs {
		if r.ID == recordID {
			return r.Content
		}
	}
	return ""
}

// logChange 写入一条解析记录变更审计日志（归属当前用户）。
func (h *DnsHandler) logChange(c *gin.Context, accountID, zone, action, recType, recName, before, after string) {
	uid, _ := c.Get("uid")
	aid, _ := strconv.ParseUint(accountID, 10, 64)
	h.db.Create(&model.DnsRecordLog{
		UserID:        uid.(uint),
		DnsAccountID:  uint(aid),
		Zone:          zone,
		Action:        action,
		RecordType:    recType,
		RecordName:    recName,
		ContentBefore: before,
		ContentAfter:  after,
	})
}

// ListLogs GET /record-logs 或 /dns-accounts/:id/zones/:zone/records/logs
// 返回当前用户最近的解析记录变更日志（可按账户/zone 过滤），并附带账户名称。
func (h *DnsHandler) ListLogs(c *gin.Context) {
	uid, _ := c.Get("uid")
	var logs []model.DnsRecordLog
	q := h.db.Where("user_id = ?", uid).Order("created_at desc").Limit(300)
	if acc := c.Query("account"); acc != "" {
		q = q.Where("dns_account_id = ?", acc)
	}
	if zone := c.Query("zone"); zone != "" {
		q = q.Where("zone = ?", zone)
	}
	q.Find(&logs)

	var accs []model.DnsAccount
	h.db.Where("user_id = ?", uid).Find(&accs)
	nameMap := make(map[uint]string, len(accs))
	for _, a := range accs {
		nameMap[a.ID] = a.Name
	}

	out := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		out = append(out, gin.H{
			"id":             l.ID,
			"account_name":   nameMap[l.DnsAccountID],
			"zone":           l.Zone,
			"action":         l.Action,
			"record_type":    l.RecordType,
			"record_name":    l.RecordName,
			"content_before": l.ContentBefore,
			"content_after":  l.ContentAfter,
			"created_at":     l.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": out})
}
