package handler

import (
	"net/http"
	"strings"

	"domain-lite/internal/config"
	"domain-lite/internal/crypto"
	"domain-lite/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DnsAccountHandler 管理 DNS 服务商账户凭据（密钥加密存储）。
type DnsAccountHandler struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewDnsAccountHandler(cfg *config.Config, db *gorm.DB) *DnsAccountHandler {
	return &DnsAccountHandler{cfg: cfg, db: db}
}

type dnsAccountReq struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Ext       string `json:"ext"`
}

// List 返回当前用户的 DNS 账户列表，密钥脱敏。
func (h *DnsAccountHandler) List(c *gin.Context) {
	uid, _ := c.Get("uid")
	var accounts []model.DnsAccount
	h.db.Where("user_id = ?", uid).Find(&accounts)
	out := make([]gin.H, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, gin.H{
			"id":         a.ID,
			"type":       a.Type,
			"name":       a.Name,
			"access_key": maskKey(a.AccessKey),
			"created_at": a.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": out})
}

// Create 新增 DNS 账户，密钥加密落库。
func (h *DnsAccountHandler) Create(c *gin.Context) {
	uid, _ := c.Get("uid")
	var req dnsAccountReq
	// access_key 允许为空：Cloudflare 的 API Token 模式不需要 email；DNSPod 的 login_token 放在 secret 里。
	// godaddy / namecheap / spaceship 需要 access_key。
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.SecretKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	if (req.Type == "godaddy" || req.Type == "namecheap" || req.Type == "spaceship") && strings.TrimSpace(req.AccessKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "该服务商类型需要填写 AccessKey"})
		return
	}
	enc, err := crypto.Encrypt(strings.TrimSpace(req.SecretKey), h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "encrypt failed"})
		return
	}
	acc := model.DnsAccount{
		UserID:    uid.(uint),
		Type:      req.Type,
		Name:      req.Name,
		AccessKey: strings.TrimSpace(req.AccessKey),
		SecretEnc: enc,
		Ext:       strings.TrimSpace(req.Ext),
	}
	h.db.Create(&acc)
	InvalidateZoneCache(uid)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"id": acc.ID}})
}

// Delete 删除 DNS 账户。
func (h *DnsAccountHandler) Delete(c *gin.Context) {
	uid, _ := c.Get("uid")
	res := h.db.Where("user_id = ? AND id = ?", uid, c.Param("id")).Delete(&model.DnsAccount{})
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
		return
	}
	InvalidateZoneCache(uid)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func maskKey(k string) string {
	if len(k) <= 4 {
		return "****"
	}
	return k[:2] + "****" + k[len(k)-2:]
}
