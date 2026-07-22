package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// namecheapProvider 实现 Namecheap API（xml.response）。
// 鉴权：ApiUser/ApiKey/UserName/ClientIp + HMAC-SHA1 签名的 Signature。
type namecheapProvider struct {
	apiUser  string
	apiKey   string
	userName string
	clientIp string
	client   *http.Client
	base     string
}

func newNamecheap(apiUser, apiKey, userName, clientIp string) DnsProvider {
	return &namecheapProvider{
		apiUser:  apiUser,
		apiKey:   apiKey,
		userName: userName,
		clientIp: clientIp,
		client:   &http.Client{Timeout: 15 * time.Second},
		base:     "https://api.namecheap.com",
	}
}

// ncApiResponse 覆盖 getList / getHosts / addHost / editHost / delHost 的响应结构。
type ncApiResponse struct {
	XMLName xml.Name `xml:"ApiResponse"`
	Status  string   `xml:"Status,attr"`
	Errors  []struct {
		Message string `xml:"Message,attr"`
	} `xml:"Errors>Error"`
	Domains []struct {
		Name string `xml:"Name,attr"`
	} `xml:"CommandResponse>DomainGetListResult>Domain"`
	Hosts []struct {
		HostId  string `xml:"HostId,attr"`
		Name    string `xml:"Name,attr"`
		Type    string `xml:"Type,attr"`
		Address string `xml:"Address,attr"`
		TTL     int    `xml:"TTL,attr"`
		MXPref  int    `xml:"MXPref,attr"`
	} `xml:"CommandResponse>DomainDNSGetHostsResult>host"`
}

// ncSign 按 Namecheap 规范：把除 Signature 外的参数按 key 排序拼成 k=v&k=v，
// 用 ApiKey 做 HMAC-SHA1 并 base64 编码。
func ncSign(apiKey string, v url.Values) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(v.Get(k))
	}
	mac := hmac.New(sha1.New, []byte(apiKey))
	mac.Write([]byte(sb.String()))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// call 组装并发送 Namecheap 请求，返回响应体（已校验 Status=OK）。
func (p *namecheapProvider) call(ctx context.Context, command string, extra map[string]string) ([]byte, error) {
	v := url.Values{}
	v.Set("ApiUser", p.apiUser)
	v.Set("ApiKey", p.apiKey)
	v.Set("UserName", p.userName)
	v.Set("ClientIp", p.clientIp)
	v.Set("Command", command)
	for k, val := range extra {
		v.Set(k, val)
	}
	v.Set("Signature", ncSign(p.apiKey, v))

	u := p.base + "/xml.response?" + v.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("User-Agent", "domain-lite/1.0")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var ar ncApiResponse
	if err := xml.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("namecheap parse %s: %w (body: %s)", command, err, string(body))
	}
	if ar.Status != "OK" {
		msg := ""
		if len(ar.Errors) > 0 {
			msg = ar.Errors[0].Message
		}
		return nil, fmt.Errorf("namecheap %s: %s", command, msg)
	}
	return body, nil
}

// splitDomain 把域名拆成 SLD/TLD（按最后一个点）。多段 TLD（如 co.uk）会近似处理。
func splitDomain(domain string) (sld, tld string) {
	domain = strings.TrimSuffix(domain, ".")
	i := strings.LastIndex(domain, ".")
	if i < 0 {
		return domain, ""
	}
	return domain[:i], domain[i+1:]
}

func (p *namecheapProvider) ListZones(ctx context.Context) ([]Zone, error) {
	body, err := p.call(ctx, "namecheap.domains.getList", nil)
	if err != nil {
		return nil, err
	}
	var ar ncApiResponse
	xml.Unmarshal(body, &ar)
	out := make([]Zone, 0, len(ar.Domains))
	for _, d := range ar.Domains {
		out = append(out, Zone{ID: d.Name, Name: d.Name})
	}
	return out, nil
}

func (p *namecheapProvider) ListRecords(ctx context.Context, zoneID string) ([]Record, error) {
	sld, tld := splitDomain(zoneID)
	body, err := p.call(ctx, "namecheap.domains.dns.getHosts", map[string]string{"SLD": sld, "TLD": tld})
	if err != nil {
		return nil, err
	}
	var ar ncApiResponse
	xml.Unmarshal(body, &ar)
	out := make([]Record, 0, len(ar.Hosts))
	for _, h := range ar.Hosts {
		out = append(out, Record{
			ID:       h.HostId,
			Name:     h.Name,
			Type:     h.Type,
			Content:  h.Address,
			TTL:      h.TTL,
			Priority: h.MXPref,
		})
	}
	return out, nil
}

// setHosts 把完整记录列表整组写回（替换该域名下的全部解析记录）。
// 注意：Namecheap 的 addHost/editHost/delHost 均为整组替换语义，单条调用会清空其它记录，
// 因此所有写操作统一走 getHosts → 本地增删改 → setHosts 的模式，保证不丢失既有记录。
func (p *namecheapProvider) setHosts(ctx context.Context, zoneID string, recs []Record) error {
	if len(recs) == 0 {
		return fmt.Errorf("namecheap setHosts: empty host list not allowed")
	}
	sld, tld := splitDomain(zoneID)
	extra := map[string]string{"SLD": sld, "TLD": tld}
	for i, r := range recs {
		n := strconv.Itoa(i + 1)
		ttl := r.TTL
		if ttl <= 0 {
			ttl = 1800
		}
		extra["HostName"+n] = r.Name
		extra["RecordType"+n] = r.Type
		extra["Address"+n] = r.Content
		extra["TTL"+n] = strconv.Itoa(ttl)
		if strings.EqualFold(r.Type, "MX") && r.Priority > 0 {
			extra["MXPref"+n] = strconv.Itoa(r.Priority)
		}
	}
	_, err := p.call(ctx, "namecheap.domains.dns.setHosts", extra)
	return err
}

func (p *namecheapProvider) CreateRecord(ctx context.Context, zoneID string, r Record) (string, error) {
	// 先取全量，追加新记录后再整组写回，避免 addHost 替换语义清空其它记录。
	recs, err := p.ListRecords(ctx, zoneID)
	if err != nil {
		return "", err
	}
	recs = append(recs, r)
	if err := p.setHosts(ctx, zoneID, recs); err != nil {
		return "", err
	}
	// 重新列出以拿到新记录的 HostId（setHosts 不直接返回稳定 ID）
	updated, err := p.ListRecords(ctx, zoneID)
	if err != nil {
		return "", err
	}
	for _, rec := range updated {
		if rec.Name == r.Name && rec.Type == r.Type && rec.Content == r.Content {
			return rec.ID, nil
		}
	}
	return "", nil
}

func (p *namecheapProvider) UpdateRecord(ctx context.Context, zoneID string, recordID string, r Record) error {
	recs, err := p.ListRecords(ctx, zoneID)
	if err != nil {
		return err
	}
	found := false
	for i := range recs {
		if recs[i].ID == recordID {
			recs[i] = r
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("namecheap update: record %s not found", recordID)
	}
	return p.setHosts(ctx, zoneID, recs)
}

func (p *namecheapProvider) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	recs, err := p.ListRecords(ctx, zoneID)
	if err != nil {
		return err
	}
	remaining := make([]Record, 0, len(recs))
	deleted := false
	for _, rec := range recs {
		if rec.ID == recordID {
			deleted = true
			continue
		}
		remaining = append(remaining, rec)
	}
	if !deleted {
		return fmt.Errorf("namecheap delete: record %s not found", recordID)
	}
	// 删到最后一条时 setHosts 不允许空列表，改用 delHost 真正删除单条记录。
	if len(remaining) == 0 {
		sld, tld := splitDomain(zoneID)
		_, err := p.call(ctx, "namecheap.domains.dns.delHost", map[string]string{"SLD": sld, "TLD": tld, "HostId": recordID})
		return err
	}
	return p.setHosts(ctx, zoneID, remaining)
}
