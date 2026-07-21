package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const cloudflareBase = "https://api.cloudflare.com/client/v4"

type cloudflareProvider struct {
	token  string
	client *http.Client
	base   string
}

// NewCloudflare 用 API Token 构造 Cloudflare provider。
// 仅使用 API Token（Authorization: Bearer），不发送 X-Auth-Email，
// 因为 Cloudflare 不允许 Bearer 与 X-Auth-Email 同时出现（会报 10000 混合鉴权错误）。
// 可通过环境变量 CF_API_BASE 覆盖 API 地址（用于本地联调/代理）。
func NewCloudflare(token string) DnsProvider {
	base := cloudflareBase
	if v := os.Getenv("CF_API_BASE"); v != "" {
		base = v
	}
	return &cloudflareProvider{
		token:  token,
		client: &http.Client{Timeout: 15 * time.Second},
		base:   base,
	}
}

type cfEnvelope struct {
	Success  bool            `json:"success"`
	Errors   []cfError       `json:"errors"`
	Result   json.RawMessage `json:"result"`
	Messages []cfError       `json:"messages"`
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (p *cloudflareProvider) do(ctx context.Context, method, path string, body interface{}) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var env cfEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("cloudflare: bad response: %w", err)
	}
	if !env.Success {
		msg := "unknown cloudflare error"
		if len(env.Errors) > 0 {
			e := env.Errors[0]
			if e.Code != 0 {
				msg = fmt.Sprintf("[CF %d] %s", e.Code, e.Message)
			} else {
				msg = e.Message
			}
		}
		log.Printf("cloudflare API error: path=%s %s", path, msg)
		return nil, fmt.Errorf("%s", msg)
	}
	return env.Result, nil
}

type cfZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cfRecord struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority"`
	Proxied  *bool  `json:"proxied"`
}

func (p *cloudflareProvider) ListZones(ctx context.Context) ([]Zone, error) {
	raw, err := p.do(ctx, "GET", "/zones?per_page=200", nil)
	if err != nil {
		return nil, err
	}
	var zs []cfZone
	if err := json.Unmarshal(raw, &zs); err != nil {
		return nil, err
	}
	out := make([]Zone, 0, len(zs))
	for _, z := range zs {
		out = append(out, Zone{ID: z.ID, Name: z.Name})
	}
	return out, nil
}

func (p *cloudflareProvider) ListRecords(ctx context.Context, zoneID string) ([]Record, error) {
	raw, err := p.do(ctx, "GET", "/zones/"+zoneID+"/dns_records?per_page=200", nil)
	if err != nil {
		return nil, err
	}
	var rs []cfRecord
	if err := json.Unmarshal(raw, &rs); err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rs))
	for _, r := range rs {
		out = append(out, Record{ID: r.ID, Name: r.Name, Type: r.Type, Content: r.Content, TTL: r.TTL, Priority: r.Priority, Proxied: r.Proxied})
	}
	return out, nil
}

func (p *cloudflareProvider) CreateRecord(ctx context.Context, zoneID string, r Record) (string, error) {
	ttl := r.TTL
	if ttl == 0 {
		ttl = 1
	}
	body := map[string]interface{}{
		"name":    r.Name,
		"type":    r.Type,
		"content": r.Content,
		"ttl":     ttl,
	}
	if r.Proxied != nil {
		body["proxied"] = *r.Proxied
	}
	if r.Priority > 0 {
		body["priority"] = r.Priority
	}
	raw, err := p.do(ctx, "POST", "/zones/"+zoneID+"/dns_records", body)
	if err != nil {
		return "", err
	}
	var z cfRecord
	if err := json.Unmarshal(raw, &z); err != nil {
		return "", err
	}
	return z.ID, nil
}

func (p *cloudflareProvider) UpdateRecord(ctx context.Context, zoneID, recordID string, r Record) error {
	ttl := r.TTL
	if ttl == 0 {
		ttl = 1
	}
	body := map[string]interface{}{
		"name":    r.Name,
		"type":    r.Type,
		"content": r.Content,
		"ttl":     ttl,
	}
	if r.Proxied != nil {
		body["proxied"] = *r.Proxied
	}
	if r.Priority > 0 {
		body["priority"] = r.Priority
	}
	_, err := p.do(ctx, "PUT", "/zones/"+zoneID+"/dns_records/"+recordID, body)
	return err
}

func (p *cloudflareProvider) DeleteRecord(ctx context.Context, zoneID, recordID string) error {
	_, err := p.do(ctx, "DELETE", "/zones/"+zoneID+"/dns_records/"+recordID, nil)
	return err
}
