package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 保存运行期配置，全部来自环境变量，便于 Docker 部署。
type Config struct {
	Port      string
	JWTSecret string
	DBPath    string
	TokenTTL  time.Duration
	// 定时同步与临期通知
	SyncIntervalMin  int    // 自动同步 WHOIS 的周期（分钟），默认 720（12h）
	NotifyEnabled    bool   // 是否启用临期 Webhook 提醒
	NotifyType       string // feishu | wecom | telegram | generic
	NotifyWebhookURL string // Webhook 地址
	NotifyChatID     string // telegram 需要
	NotifyThreshold  int    // 剩余天数 <= 该值则提醒，默认 30
}

func Load() *Config {
	ttlMin := getenvInt("TOKEN_TTL_MINUTES", 7*24*60)
	c := &Config{
		Port:      getenv("PORT", "8080"),
		JWTSecret: os.Getenv("JWT_SECRET"),
		DBPath:    getenv("DB_PATH", "data/domain-lite.db"),
		TokenTTL:  time.Duration(ttlMin) * time.Minute,
		// 定时同步与临期通知（均可通过环境变量覆盖）
		SyncIntervalMin:  getenvInt("SYNC_INTERVAL_MIN", 720),
		NotifyEnabled:    getenvBool("NOTIFY_ENABLED", false),
		NotifyType:       getenv("NOTIFY_TYPE", "feishu"),
		NotifyWebhookURL: getenv("NOTIFY_WEBHOOK_URL", ""),
		NotifyChatID:     getenv("NOTIFY_CHAT_ID", ""),
		NotifyThreshold:  getenvInt("NOTIFY_THRESHOLD_DAYS", 30),
	}

	// JWT_SECRET 是签发/校验 token 的密钥，绝不能使用镜像内置的默认值。
	// 缺失时直接启动失败，避免静默使用空/弱密钥导致的安全风险。
	if c.JWTSecret == "" {
		log.Fatal("JWT_SECRET 环境变量未设置：请通过 -e JWT_SECRET=... 或环境变量提供（不能为空），否则无法安全签发登录 token")
	}
	return c
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		return strings.EqualFold(v, "true") || v == "1"
	}
	return def
}
