package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// spaceshipProvider 实现 Spaceship API（v1）。
// 鉴权：请求头 X-Api-Key / X-Api-Secret（明文，无需签名）。
// 记录无原生 ID，用 type|name|address 合成；写操作按 type+name+address 匹配。
type spaceshipProvider struct {
	apiKey    string
	apiSecret string
	client    *http.Client
	base      string
}

func newSpaceship(apiKey, apiSecret string) DnsProvider {
	return &spaceshipProvider{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		client:    &http.Client{Timeout: 15 * time.Second},
		base:      "https://spaceship.dev/api/v1",
	}
}

func (p *spaceshipProvider) auth(req *http.Request) {
	req.Header.Set("X-Api-Key", p.apiKey)
	req.Header.Set("X-Api-Secret", p.apiSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

type spaceshipRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	TTL     int    `json:"ttl"`
	Address string `json:"address"`
	Content string `json:"content"`
}

func (p *spaceshipProvider) ListZones(ctx context.Context) ([]Zone, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.base+"/domains?take=100&skip=0", nil)
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusErr("spaceship list domains", resp)
	}
	var r struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	out := make([]Zone, 0, len(r.Items))
	for _, d := range r.Items {
		out = append(out, Zone{ID: d.Name, Name: d.Name})
	}
	return out, nil
}

func (p *spaceshipProvider) ListRecords(ctx context.Context, zoneID string) ([]Record, error) {
	u := fmt.Sprintf("%s/dns/records/%s?take=500&skip=0", p.base, zoneID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusErr("spaceship list records", resp)
	}
	var r struct {
		Items []spaceshipRecord `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(r.Items))
	for _, it := range r.Items {
		content := it.Address
		if content == "" {
			content = it.Content
		}
		out = append(out, Record{
			ID:      encodeID(it.Type, it.Name, content),
			Name:    it.Name,
			Type:    it.Type,
			Content: content,
			TTL:     it.TTL,
		})
	}
	return out, nil
}

func (p *spaceshipProvider) CreateRecord(ctx context.Context, zoneID string, r Record) (string, error) {
	item := map[string]any{
		"type":    r.Type,
		"name":    r.Name,
		"address": r.Content,
		"ttl":     ttlOf(r.TTL),
	}
	body, _ := json.Marshal(map[string]any{"force": true, "items": []any{item}})
	u := fmt.Sprintf("%s/dns/records/%s", p.base, zoneID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", statusErr("spaceship create record", resp)
	}
	return encodeID(r.Type, r.Name, r.Content), nil
}

func (p *spaceshipProvider) UpdateRecord(ctx context.Context, zoneID string, recordID string, r Record) error {
	// 先按旧记录(type+name+address)删除，再 PUT 新值（Spaceship 无按 ID 更新）
	parts := decodeID(recordID)
	if len(parts) >= 3 {
		delBody, _ := json.Marshal([]map[string]string{{
			"type":    parts[0],
			"name":    parts[1],
			"address": parts[2],
		}})
		u := fmt.Sprintf("%s/dns/records/%s", p.base, zoneID)
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, u, bytes.NewReader(delBody))
		p.auth(req)
		if resp, err := p.client.Do(req); err == nil {
			resp.Body.Close()
		}
	}
	_, err := p.CreateRecord(ctx, zoneID, r)
	return err
}

func (p *spaceshipProvider) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	parts := decodeID(recordID)
	if len(parts) < 3 {
		return fmt.Errorf("spaceship: 无效的记录 ID")
	}
	delBody, _ := json.Marshal([]map[string]string{{
		"type":    parts[0],
		"name":    parts[1],
		"address": parts[2],
	}})
	u := fmt.Sprintf("%s/dns/records/%s", p.base, zoneID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, u, bytes.NewReader(delBody))
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return statusErr("spaceship delete record", resp)
	}
	return nil
}
