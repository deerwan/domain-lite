package provider

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const aliyunEndpoint = "https://alidns.aliyuncs.com/"

type aliyunProvider struct {
	accessKeyID     string
	accessKeySecret string
	client          *http.Client
	endpoint        string
}

// NewAliyun 用 AccessKeyId / AccessKeySecret 构造阿里云 DNS provider。
func NewAliyun(accessKeyID, accessKeySecret string) DnsProvider {
	return &aliyunProvider{
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		client:          &http.Client{Timeout: 15 * time.Second},
		endpoint:        aliyunEndpoint,
	}
}

// percentEncode 按阿里云 RPC 签名规则编码（空格 -> %20，其余非字母数字 -> %XX 大写）。
func percentEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func (a *aliyunProvider) sign(method, canonicalQuery string) string {
	stringToSign := method + "&" + percentEncode("/") + "&" + percentEncode(canonicalQuery)
	mac := hmac.New(sha1.New, []byte(a.accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func randHex() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// call 构造阿里云 RPC 请求（HMAC-SHA1 签名）并返回响应体。
func (a *aliyunProvider) call(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	params["Format"] = "JSON"
	params["Version"] = "2015-01-09"
	params["AccessKeyId"] = a.accessKeyID
	params["SignatureMethod"] = "HMAC-SHA1"
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = randHex()

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var canon strings.Builder
	for i, k := range keys {
		if i > 0 {
			canon.WriteByte('&')
		}
		canon.WriteString(percentEncode(k))
		canon.WriteByte('=')
		canon.WriteString(percentEncode(params[k]))
	}
	sig := a.sign("GET", canon.String())
	full := canon.String() + "&Signature=" + percentEncode(sig)
	u := a.endpoint + "?" + full

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 阿里云错误响应含顶层 Code 字段。
	var probe struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(data, &probe); err == nil && probe.Code != "" {
		return nil, fmt.Errorf("aliyun error: %s: %s", probe.Code, probe.Message)
	}
	return json.RawMessage(data), nil
}

type aliyunDomain struct {
	DomainName string `json:"DomainName"`
}

type aliyunDescribeDomainsResp struct {
	Domains struct {
		Domain []aliyunDomain `json:"Domain"`
	} `json:"Domains"`
}

type aliyunRecord struct {
	RecordId string `json:"RecordId"`
	RR       string `json:"RR"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
	Priority int    `json:"Priority"`
}

type aliyunDescribeRecordsResp struct {
	DomainRecords struct {
		Record []aliyunRecord `json:"Record"`
	} `json:"DomainRecords"`
}

func (a *aliyunProvider) ListZones(ctx context.Context) ([]Zone, error) {
	raw, err := a.call(ctx, map[string]string{"Action": "DescribeDomains", "PageSize": "100"})
	if err != nil {
		return nil, err
	}
	var resp aliyunDescribeDomainsResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	out := make([]Zone, 0, len(resp.Domains.Domain))
	for _, d := range resp.Domains.Domain {
		out = append(out, Zone{ID: d.DomainName, Name: d.DomainName})
	}
	return out, nil
}

func (a *aliyunProvider) ListRecords(ctx context.Context, zoneID string) ([]Record, error) {
	raw, err := a.call(ctx, map[string]string{"Action": "DescribeDomainRecords", "DomainName": zoneID, "PageSize": "100"})
	if err != nil {
		return nil, err
	}
	var resp aliyunDescribeRecordsResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(resp.DomainRecords.Record))
	for _, r := range resp.DomainRecords.Record {
		out = append(out, Record{ID: r.RecordId, Name: r.RR, Type: r.Type, Content: r.Value, TTL: r.TTL, Priority: r.Priority})
	}
	return out, nil
}

func (a *aliyunProvider) CreateRecord(ctx context.Context, zoneID string, r Record) (string, error) {
	ttl := r.TTL
	if ttl == 0 {
		ttl = 600
	}
	params := map[string]string{
		"Action":     "AddDomainRecord",
		"DomainName": zoneID,
		"RR":         r.Name,
		"Type":       r.Type,
		"Value":      r.Content,
		"TTL":        fmt.Sprintf("%d", ttl),
	}
	if r.Priority > 0 {
		params["Priority"] = fmt.Sprintf("%d", r.Priority)
	}
	raw, err := a.call(ctx, params)
	if err != nil {
		return "", err
	}
	var resp struct {
		RecordId string `json:"RecordId"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	return resp.RecordId, nil
}

func (a *aliyunProvider) UpdateRecord(ctx context.Context, zoneID, recordID string, r Record) error {
	ttl := r.TTL
	if ttl == 0 {
		ttl = 600
	}
	params := map[string]string{
		"Action":   "UpdateDomainRecord",
		"RecordId": recordID,
		"RR":       r.Name,
		"Type":     r.Type,
		"Value":    r.Content,
		"TTL":      fmt.Sprintf("%d", ttl),
	}
	if r.Priority > 0 {
		params["Priority"] = fmt.Sprintf("%d", r.Priority)
	}
	_, err := a.call(ctx, params)
	return err
}

func (a *aliyunProvider) DeleteRecord(ctx context.Context, zoneID, recordID string) error {
	_, err := a.call(ctx, map[string]string{"Action": "DeleteDomainRecord", "RecordId": recordID})
	return err
}
