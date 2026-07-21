package model

import "time"

// User 系统用户（轻量自用，默认单管理员）。
type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	Role         string `gorm:"default:admin"`
	CreatedAt    time.Time
}

// DnsAccount DNS 服务商账户凭据。SecretKey 加密后存入 SecretEnc，明文不落库。
type DnsAccount struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index"`
	Type      string `gorm:"index"` // cloudflare | aliyun | godaddy | dnspod | namecheap | spaceship
	Name      string // 备注名
	AccessKey string // cloudflare: email(可选) / aliyun: AccessKeyId / godaddy: API Key / namecheap: ApiUser / dnspod: 空(login_token 在 secret) / spaceship: Api Key
	SecretEnc string // 加密存储的密钥
	Ext       string `json:"ext"` // 服务商特定扩展参数(JSON)，如 namecheap 的 client_ip
	CreatedAt time.Time
}

// Domain 域名集合，缓存 WHOIS 与 DNS 解析结果用于列表展示。
type Domain struct {
	ID           uint   `gorm:"primaryKey"`
	UserID       uint   `gorm:"index"`
	DnsAccountID uint   `gorm:"index"` // 关联的 DNS 账户（可选）
	Domain       string `gorm:"index"`
	ZoneID       string // 服务商处的 zone 标识
	Note         string // 备注
	// WHOIS 缓存
	Registrar string
	ExpireAt  *time.Time
	Status    string
	// 临期通知去重：最近一次发送提醒的时间
	LastNotifiedAt *time.Time
	// DNS 解析缓存（JSON 字符串）
	DnsRecords string
	LastCheck  *time.Time
	CreatedAt  time.Time
}

// NotifySetting 每用户的通知与自动同步配置（前端可编辑，覆盖 env 默认值）。
// WebhookURLEnc 加密存储，明文不落库。
type NotifySetting struct {
	ID              uint   `gorm:"primaryKey"`
	UserID          uint   `gorm:"uniqueIndex;not null"`
	Enabled         bool   `json:"enabled"`
	Type            string `json:"type"` // feishu | wecom | telegram | generic
	WebhookURLEnc   string `json:"-"`    // 加密存储的 Webhook 地址
	ChatID          string `json:"chat_id"`
	ThresholdDays   int    `json:"threshold_days"`    // 临期提醒阈值（天）
	SyncIntervalMin int    `json:"sync_interval_min"` // 自动同步间隔（分钟）
	UpdatedAt       time.Time
}

// DnsRecordLog 解析记录变更审计日志（创建/更新/删除）。
type DnsRecordLog struct {
	ID            uint   `gorm:"primaryKey"`
	UserID        uint   `gorm:"index;not null"`
	DnsAccountID  uint   `gorm:"index"`
	Zone          string `gorm:"index"`
	Action        string // create | update | delete
	RecordType    string // A / CNAME / MX ...
	RecordName    string // 记录名（含域名或 @）
	ContentBefore string `json:"content_before"`
	ContentAfter  string `json:"content_after"`
	CreatedAt     time.Time
}
