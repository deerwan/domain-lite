package provider

import "context"

// Zone 表示服务商侧的一个托管域。
// 注意 JSON tag 必须为小写，与前端 Zone 类型（{id,name}）保持一致。
type Zone struct {
	ID   string `json:"id"` // Cloudflare: zone id；阿里云: 域名本身
	Name string `json:"name"`
}

// Record 跨服务商统一的 DNS 记录结构。
// JSON tag 小写，与前端 DnsRecord 类型保持一致。
type Record struct {
	ID       string `json:"id"`
	Name     string `json:"name"` // Cloudflare: 完整主机名；阿里云: RR（相对主机名，如 www / @）
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority"`
	Proxied  *bool  `json:"proxied"` // Cloudflare 专属（orange cloud），其他服务商忽略
}

// DnsProvider 单个 DNS 账户的操作接口。
type DnsProvider interface {
	ListZones(ctx context.Context) ([]Zone, error)
	ListRecords(ctx context.Context, zoneID string) ([]Record, error)
	CreateRecord(ctx context.Context, zoneID string, r Record) (string, error)
	UpdateRecord(ctx context.Context, zoneID, recordID string, r Record) error
	DeleteRecord(ctx context.Context, zoneID, recordID string) error
}
