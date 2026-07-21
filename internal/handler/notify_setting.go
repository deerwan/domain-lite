package handler

import (
	"net/http"
	"strings"

	"domain-lite/internal/config"
	"domain-lite/internal/crypto"
	"domain-lite/internal/model"
	"domain-lite/internal/notify"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NotifySettingHandler 管理每用户的通知与自动同步配置（前端可编辑，覆盖 env 默认值）。
type NotifySettingHandler struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewNotifySettingHandler(cfg *config.Config, db *gorm.DB) *NotifySettingHandler {
	return &NotifySettingHandler{cfg: cfg, db: db}
}

type notifySettingReq struct {
	Enabled         bool   `json:"enabled"`
	Type            string `json:"type"`
	WebhookURL      string `json:"webhook_url"` // 空表示保留原值
	ChatID          string `json:"chat_id"`
	ThresholdDays   int    `json:"threshold_days"`
	SyncIntervalMin int    `json:"sync_interval_min"`
}

// Get 返回当前用户的通知配置；若未设置过则返回 env 默认值，Webhook 地址脱敏。
func (h *NotifySettingHandler) Get(c *gin.Context) {
	uid, _ := c.Get("uid")
	out := gin.H{
		"enabled":           h.cfg.NotifyEnabled,
		"type":              h.cfg.NotifyType,
		"chat_id":           h.cfg.NotifyChatID,
		"threshold_days":    h.cfg.NotifyThreshold,
		"sync_interval_min": h.cfg.SyncIntervalMin,
		"has_webhook":       false,
		"webhook_masked":    "",
	}
	var s model.NotifySetting
	if err := h.db.Where("user_id = ?", uid).First(&s).Error; err == nil {
		out["enabled"] = s.Enabled
		out["type"] = s.Type
		out["chat_id"] = s.ChatID
		out["threshold_days"] = s.ThresholdDays
		out["sync_interval_min"] = s.SyncIntervalMin
		if s.WebhookURLEnc != "" {
			out["has_webhook"] = true
			if url, derr := crypto.Decrypt(s.WebhookURLEnc, h.cfg.JWTSecret); derr == nil {
				out["webhook_masked"] = maskURL(url)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": out})
}

// Update 新增或更新当前用户的通知配置；webhook_url 传空表示沿用已保存的值。
func (h *NotifySettingHandler) Update(c *gin.Context) {
	uid, _ := c.Get("uid")
	var req notifySettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	if req.Type == "" {
		req.Type = h.cfg.NotifyType
	}
	if req.ThresholdDays <= 0 {
		req.ThresholdDays = h.cfg.NotifyThreshold
	}
	if req.SyncIntervalMin <= 0 {
		req.SyncIntervalMin = h.cfg.SyncIntervalMin
	}

	var s model.NotifySetting
	if err := h.db.Where("user_id = ?", uid).First(&s).Error; err != nil {
		s = model.NotifySetting{UserID: uid.(uint)}
		h.db.Create(&s)
	}

	updates := map[string]any{
		"enabled":           req.Enabled,
		"type":              req.Type,
		"chat_id":           req.ChatID,
		"threshold_days":    req.ThresholdDays,
		"sync_interval_min": req.SyncIntervalMin,
	}
	if strings.TrimSpace(req.WebhookURL) != "" {
		enc, err := crypto.Encrypt(strings.TrimSpace(req.WebhookURL), h.cfg.JWTSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "encrypt failed"})
			return
		}
		updates["webhook_url_enc"] = enc
	}
	h.db.Model(&s).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// Test 使用当前保存或表单传入的 Webhook 配置发送一条测试消息。
func (h *NotifySettingHandler) Test(c *gin.Context) {
	uid, _ := c.Get("uid")
	var req notifySettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	cfg := notify.Config{
		Enabled:       true,
		Type:          req.Type,
		WebhookURL:    strings.TrimSpace(req.WebhookURL),
		ChatID:        req.ChatID,
		ThresholdDays: req.ThresholdDays,
	}
	if cfg.WebhookURL == "" {
		var s model.NotifySetting
		if h.db.Where("user_id = ?", uid).First(&s).Error == nil && s.WebhookURLEnc != "" {
			if url, derr := crypto.Decrypt(s.WebhookURLEnc, h.cfg.JWTSecret); derr == nil {
				cfg.WebhookURL = url
			}
		}
	}
	if cfg.Type == "" {
		cfg.Type = h.cfg.NotifyType
	}
	if cfg.Type == "" {
		cfg.Type = "feishu"
	}
	if cfg.WebhookURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "未配置 Webhook 地址"})
		return
	}
	if err := notify.Send(cfg, "✅ 通知测试", []string{"这是一条来自 domain-lite 的测试消息，说明 Webhook 配置正确。"}); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "发送失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "发送成功"})
}

// maskURL 对 Webhook 地址做脱敏，仅保留头尾少量字符。
func maskURL(u string) string {
	if len(u) <= 12 {
		return strings.Repeat("*", len(u))
	}
	return u[:8] + "****" + u[len(u)-4:]
}
