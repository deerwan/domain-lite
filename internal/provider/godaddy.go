package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// godaddyProvider 实现 GoDaddy 开发者 API（v1）。
// 鉴权：请求头 Authorization: sso-key <key>:<secret>，无需签名。
type godaddyProvider struct {
	key    string
	secret string
	client *http.Client
	base   string
}

func newGodaddy(key, secret string) DnsProvider {
	return &godaddyProvider{
		key:    key,
		secret: secret,
		client: &http.Client{Timeout: 15 * time.Second},
		base:   "https://api.godaddy.com/v1",
	}
}

func (p *godaddyProvider) auth(req *http.Request) {
	req.Header.Set("Authorization", "sso-key "+p.key+":"+p.secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

type godaddyDomain struct {
	Domain string `json:"domain"`
}

type godaddyRecord struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	TTL      int      `json:"ttl"`
	Data     []string `json:"data"`
	Priority int      `json:"priority"`
}

func (p *godaddyProvider) ListZones(ctx context.Context) ([]Zone, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.base+"/domains?statuses=ACTIVE", nil)
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusErr("godaddy list domains", resp)
	}
	var ds []godaddyDomain
	if err := json.NewDecoder(resp.Body).Decode(&ds); err != nil {
		return nil, err
	}
	out := make([]Zone, 0, len(ds))
	for _, d := range ds {
		out = append(out, Zone{ID: d.Domain, Name: d.Domain})
	}
	return out, nil
}

func (p *godaddyProvider) ListRecords(ctx context.Context, zoneID string) ([]Record, error) {
	url := fmt.Sprintf("%s/domains/%s/records", p.base, zoneID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusErr("godaddy list records", resp)
	}
	body, _ := io.ReadAll(resp.Body)
	records := []Record{}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "{") {
		// 按类型分组的对象：{"A":[{...}],"MX":[{...}]}
		byType := map[string][]godaddyRecord{}
		if err := json.Unmarshal(body, &byType); err == nil {
			for _, arr := range byType {
				for _, r := range arr {
					records = append(records, godaddyToRecord(r))
				}
			}
		}
	} else {
		// 数组形式
		var arr []godaddyRecord
		if err := json.Unmarshal(body, &arr); err == nil {
			for _, r := range arr {
				records = append(records, godaddyToRecord(r))
			}
		}
	}
	return records, nil
}

func godaddyToRecord(r godaddyRecord) Record {
	content := strings.Join(r.Data, ",")
	return Record{
		ID:       encodeID(r.Type, r.Name, content),
		Name:     r.Name,
		Type:     r.Type,
		Content:  content,
		TTL:      r.TTL,
		Priority: r.Priority,
	}
}

func (p *godaddyProvider) CreateRecord(ctx context.Context, zoneID string, r Record) (string, error) {
	rec := godaddyRecord{Type: r.Type, Name: r.Name, TTL: ttlOf(r.TTL), Data: splitContent(r.Content)}
	body, _ := json.Marshal([]godaddyRecord{rec})
	url := fmt.Sprintf("%s/domains/%s/records", p.base, zoneID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", statusErr("godaddy create record", resp)
	}
	return encodeID(r.Type, r.Name, r.Content), nil
}

func (p *godaddyProvider) UpdateRecord(ctx context.Context, zoneID string, recordID string, r Record) error {
	parts := decodeID(recordID)
	if len(parts) < 2 {
		return fmt.Errorf("godaddy: 无效的记录 ID")
	}
	typ, name := parts[0], parts[1]
	rec := godaddyRecord{Type: r.Type, Name: r.Name, TTL: ttlOf(r.TTL), Data: splitContent(r.Content)}
	body, _ := json.Marshal(rec)
	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", p.base, zoneID, typ, name)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return statusErr("godaddy update record", resp)
	}
	return nil
}

func (p *godaddyProvider) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	parts := decodeID(recordID)
	if len(parts) < 2 {
		return fmt.Errorf("godaddy: 无效的记录 ID")
	}
	typ, name := parts[0], parts[1]
	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", p.base, zoneID, typ, name)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return statusErr("godaddy delete record", resp)
	}
	return nil
}
