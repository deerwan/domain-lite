package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// dnspodProvider 实现 DNSPod 旧版用户 API（https://dnsapi.cn）。
// 鉴权：login_token（格式 "ID,Token"），POST 表单提交，需要 User-Agent。
type dnspodProvider struct {
	token  string // 完整 login_token: "ID,Token"
	client *http.Client
	base   string
}

func newDNSPod(token string) DnsProvider {
	return &dnspodProvider{
		token:  token,
		client: &http.Client{Timeout: 15 * time.Second},
		base:   "https://dnsapi.cn",
	}
}

func (p *dnspodProvider) post(ctx context.Context, action string, form url.Values) ([]byte, error) {
	if form == nil {
		form = url.Values{}
	}
	form.Set("login_token", p.token)
	form.Set("format", "json")
	form.Set("lang", "cn")
	form.Set("error_on_empty", "no")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/"+action, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "domain-lite/1.0 (admin)")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dnspod %s: %s: %s", action, resp.Status, string(body))
	}
	var sr struct {
		Status struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &sr); err == nil && sr.Status.Code != 1 {
		return nil, fmt.Errorf("dnspod %s: %s", action, sr.Status.Message)
	}
	return body, nil
}

func (p *dnspodProvider) ListZones(ctx context.Context) ([]Zone, error) {
	body, err := p.post(ctx, "Domain.List", url.Values{})
	if err != nil {
		return nil, err
	}
	var r struct {
		Domains []struct {
			ID   json.Number `json:"id"`
			Name string      `json:"name"`
		} `json:"domains"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	out := make([]Zone, 0, len(r.Domains))
	for _, d := range r.Domains {
		out = append(out, Zone{ID: d.ID.String(), Name: d.Name})
	}
	return out, nil
}

func (p *dnspodProvider) ListRecords(ctx context.Context, zoneID string) ([]Record, error) {
	form := url.Values{}
	form.Set("domain", zoneID)
	body, err := p.post(ctx, "Record.List", form)
	if err != nil {
		return nil, err
	}
	var r struct {
		Records []struct {
			ID      json.Number `json:"id"`
			Name    string      `json:"name"`
			Type    string      `json:"type"`
			Value   string      `json:"value"`
			TTL     json.Number `json:"ttl"`
			MX      json.Number `json:"mx"`
			Enabled string      `json:"enabled"`
		} `json:"records"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(r.Records))
	for _, rec := range r.Records {
		ttl, _ := rec.TTL.Int64()
		mx, _ := rec.MX.Int64()
		out = append(out, Record{
			ID:       rec.ID.String(),
			Name:     rec.Name,
			Type:     rec.Type,
			Content:  rec.Value,
			TTL:      int(ttl),
			Priority: int(mx),
		})
	}
	return out, nil
}

func (p *dnspodProvider) CreateRecord(ctx context.Context, zoneID string, r Record) (string, error) {
	form := url.Values{}
	form.Set("domain", zoneID)
	form.Set("sub_domain", r.Name)
	form.Set("record_type", r.Type)
	form.Set("record_line", "默认")
	form.Set("value", r.Content)
	if r.TTL > 0 {
		form.Set("ttl", strconv.Itoa(r.TTL))
	}
	if r.Priority > 0 {
		form.Set("mx", strconv.Itoa(r.Priority))
	}
	body, err := p.post(ctx, "Record.Create", form)
	if err != nil {
		return "", err
	}
	var rr struct {
		Record struct {
			ID json.Number `json:"id"`
		} `json:"record"`
	}
	if err := json.Unmarshal(body, &rr); err != nil {
		return "", err
	}
	return rr.Record.ID.String(), nil
}

func (p *dnspodProvider) UpdateRecord(ctx context.Context, zoneID string, recordID string, r Record) error {
	form := url.Values{}
	form.Set("domain", zoneID)
	form.Set("record_id", recordID)
	form.Set("sub_domain", r.Name)
	form.Set("record_type", r.Type)
	form.Set("record_line", "默认")
	form.Set("value", r.Content)
	if r.TTL > 0 {
		form.Set("ttl", strconv.Itoa(r.TTL))
	}
	if r.Priority > 0 {
		form.Set("mx", strconv.Itoa(r.Priority))
	}
	_, err := p.post(ctx, "Record.Modify", form)
	return err
}

func (p *dnspodProvider) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	form := url.Values{}
	form.Set("domain", zoneID)
	form.Set("record_id", recordID)
	_, err := p.post(ctx, "Record.Remove", form)
	return err
}
