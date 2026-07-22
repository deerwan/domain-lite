package whois

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Info 解析后的 WHOIS 核心信息。
type Info struct {
	Registrar string
	ExpireAt  *time.Time
	Status    string
	// Privacy 表示域名启用了隐私保护（注册人信息被 Redact）。
	// 此时注册商通常仍可解析，但注册人/联系方式被隐藏。
	Privacy bool
}

var (
	reReferral       = regexp.MustCompile(`(?i)whois:\s*(\S+)`)
	reExpiry         = regexp.MustCompile(`(?im)(?:Registrar Registration Expiration Date|Registry Expiry Date|Expiry Date|Expiration Date|Expires On|Expires):\s*(.+)`)
	reRegistrar      = regexp.MustCompile(`(?im)(?:Sponsoring Registrar|Registrar):\s*(.+)`)
	reRegistrarWhois = regexp.MustCompile(`(?im)Registrar WHOIS Server:\s*(\S+)`)
	reStatus         = regexp.MustCompile(`(?im)Domain Status:\s*(\S+)`)
	reTrailing       = regexp.MustCompile(`\s*\([^)]*\)\s*$`)
	// rePrivacy 检测隐私保护：WHOIS 中注册商/注册人字段被 Redact 的特征串。
	rePrivacy = regexp.MustCompile(`(?i)redacted for privacy|whoisguard|privacy service|domain privacy|not disclosed|contact privacy|identity protected|private registration|privacy protect`)
)

// Lookup 查询域名的 WHOIS 信息。
// 策略：先走传统 WHOIS(43 端口，IANA→注册局→注册商逐级 referral)；
// 若关键字段(到期日)缺失或失败，再用 RDAP(HTTP/JSON，自动处理多段 TLD 与隐私保护域名)兜底；
// 若仍查不到到期日，则逐级去掉最左标签重试到注册域(如 elk.de5.net → de5.net)，
// 覆盖「子域名/动态 DNS 主机名」场景——它们无独立 WHOIS，但其生命周期归属注册域。
// 整个过程受 ctx 超时约束；所有层级都失败才返回 error，由调用方降级处理。
func Lookup(ctx context.Context, domain string) (*Info, error) {
	labels := strings.Split(domain, ".")
	var lastErr error
	// i=0 为完整域名，逐步缩短到至少保留 2 段(注册域)。
	for i := 0; i < len(labels)-1; i++ {
		cand := strings.Join(labels[i:], ".")
		info, werr := attempt(ctx, cand)
		if info != nil && info.ExpireAt != nil {
			return info, nil
		}
		if werr != nil {
			lastErr = werr
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	// 所有层级均无到期日：用首次尝试的部分结果(可能含注册商)返回，不报错。
	if info, _ := attempt(ctx, domain); info != nil {
		return info, nil
	}
	return nil, fmt.Errorf("whois: no data for %s", domain)
}

// attempt 对单个候选域名同时走 WHOIS 与 RDAP 兜底，返回合并后的信息。
func attempt(ctx context.Context, domain string) (*Info, error) {
	info, werr := whoisLookup(ctx, domain)
	if werr == nil && info != nil && info.ExpireAt != nil {
		return info, nil // WHOIS 已拿到关键字段，直接用，最快
	}
	// 兜底：RDAP（结构化 JSON，对 .co.uk 等多段 TLD 与隐私保护域名更鲁棒）
	if rinfo, rerr := rdapLookup(ctx, domain); rerr == nil && rinfo != nil {
		if info == nil {
			return rinfo, nil
		}
		// 合并：优先保留 WHOIS 已解析出的字段，缺失的用 RDAP 补齐
		if info.ExpireAt == nil && rinfo.ExpireAt != nil {
			info.ExpireAt = rinfo.ExpireAt
		}
		if info.Registrar == "" && rinfo.Registrar != "" {
			info.Registrar = rinfo.Registrar
		}
		if info.Status == "" && rinfo.Status != "" {
			info.Status = rinfo.Status
		}
		info.Privacy = info.Privacy || rinfo.Privacy
		return info, nil
	}
	if info != nil {
		return info, nil // 至少返回 WHOIS 的部分结果
	}
	if werr != nil {
		return nil, werr
	}
	return nil, fmt.Errorf("whois: no data for %s", domain)
}

// whoisLookup 传统 WHOIS：IANA 获取注册局服务器，再查询并解析，必要时跟进注册商 referral。
func whoisLookup(ctx context.Context, domain string) (*Info, error) {
	server, err := referralServer(ctx, domain)
	if err != nil {
		return nil, err
	}
	raw, err := query(ctx, server, domain)
	if err != nil {
		return nil, err
	}
	info := parse(raw)
	// 二级 referral：部分注册局(如 Verisign)的 WHOIS 仅含概要，
	// 附带的 Registrar WHOIS Server 指向注册商，二次查询可拿到更完整的注册商/状态。
	if m := reRegistrarWhois.FindStringSubmatch(raw); len(m) >= 2 {
		if rraw, rerr := query(ctx, m[1], domain); rerr == nil {
			rinfo := parse(rraw)
			if info.Registrar == "" && rinfo.Registrar != "" {
				info.Registrar = rinfo.Registrar
			}
			if info.ExpireAt == nil && rinfo.ExpireAt != nil {
				info.ExpireAt = rinfo.ExpireAt
			}
			if info.Status == "" && rinfo.Status != "" {
				info.Status = rinfo.Status
			}
			info.Privacy = info.Privacy || rinfo.Privacy
		}
	}
	return info, nil
}

func referralServer(ctx context.Context, domain string) (string, error) {
	raw, err := query(ctx, "whois.iana.org", domain)
	if err != nil {
		return "", err
	}
	m := reReferral.FindStringSubmatch(raw)
	if len(m) < 2 {
		return "", io.EOF // 无 referral，视为查询失败
	}
	return m[1], nil
}

// query 通过 43 端口 WHOIS 协议查询。
func query(ctx context.Context, server, domain string) (string, error) {
	d := net.Dialer{Timeout: 8 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(server, "43"))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(8 * time.Second))
	if _, err := conn.Write([]byte(domain + "\r\n")); err != nil {
		return "", err
	}
	data, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parse(raw string) *Info {
	info := &Info{}
	if m := reExpiry.FindStringSubmatch(raw); len(m) >= 2 {
		if t := parseDate(m[1]); t != nil {
			info.ExpireAt = t
		}
	}
	if m := reRegistrar.FindStringSubmatch(raw); len(m) >= 2 {
		r := strings.TrimSpace(m[1])
		if rePrivacy.MatchString(r) {
			info.Privacy = true
			// 隐私服务商名仍有展示价值(如 WhoisGuard, Inc.)；纯 REDACTED 则留空待 RDAP 补齐。
			if !strings.Contains(strings.ToUpper(r), "REDACTED") {
				info.Registrar = r
			}
		} else {
			info.Registrar = r
		}
	} else if rePrivacy.MatchString(raw) {
		info.Privacy = true
	}
	if m := reStatus.FindStringSubmatch(raw); len(m) >= 2 {
		info.Status = strings.TrimSpace(m[1])
	}
	return info
}

// ---- RDAP 兜底 ----

type rdapDoc struct {
	Events   []rdapEvent  `json:"events"`
	Entities []rdapEntity `json:"entities"`
	Status   []string     `json:"status"`
}

type rdapEvent struct {
	Action string `json:"eventAction"`
	Date   string `json:"eventDate"`
}

type rdapEntity struct {
	Roles      []string     `json:"roles"`
	VCardArray []any        `json:"vcardArray"`
	Entities   []rdapEntity `json:"entities"`
}

// rdapLookup 通过 RDAP(注册数据访问协议)查询，返回结构化 JSON，天然适配多段 TLD 与隐私保护域名。
func rdapLookup(ctx context.Context, domain string) (*Info, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://rdap.org/domain/"+domain, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/rdap+json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rdap: status %d", resp.StatusCode)
	}
	var doc rdapDoc
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, err
	}
	info := &Info{}
	for _, e := range doc.Events {
		if e.Action == "expiration" {
			if t := parseDate(e.Date); t != nil {
				info.ExpireAt = t
			}
		}
	}
	if reg := rdapRegistrar(doc.Entities); reg != "" {
		info.Registrar = reg
	}
	if len(doc.Status) > 0 {
		info.Status = doc.Status[0]
	}
	if reg := info.Registrar; rePrivacy.MatchString(reg) {
		info.Privacy = true
	}
	for _, s := range doc.Status {
		if rePrivacy.MatchString(s) {
			info.Privacy = true
		}
	}
	return info, nil
}

// rdapRegistrar 递归在 entities 中查找 role=registrar 的实体，并从 vCard 提取名称(fn)。
func rdapRegistrar(entities []rdapEntity) string {
	for _, e := range entities {
		for _, role := range e.Roles {
			if role == "registrar" {
				if fn := rdapFn(e.VCardArray); fn != "" {
					return fn
				}
			}
		}
		if s := rdapRegistrar(e.Entities); s != "" {
			return s
		}
	}
	return ""
}

// rdapFn 从 RDAP 的 vCard 数组中提取 fn(全名) 字段。
func rdapFn(v []any) string {
	if len(v) < 2 {
		return ""
	}
	arr, ok := v[1].([]any)
	if !ok {
		return ""
	}
	for _, item := range arr {
		row, ok := item.([]any)
		if !ok || len(row) < 4 {
			continue
		}
		if name, ok := row[0].(string); ok && name == "fn" {
			if fn, ok := row[3].(string); ok {
				return fn
			}
		}
	}
	return ""
}

func parseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	s = reTrailing.ReplaceAllString(s, "")
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02-Jan-2006",
		"02-Jan-2006 15:04:05",
		"02 Jan 2006",
		"02 Jan 2006 15:04:05",
		"January 2 2006",
		"January 02, 2006",
		"2006/01/02",
		"2006.01.02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return &t
		}
	}
	return nil
}

// DaysUntil 距离到期天数（负数表示已过期）。
func DaysUntil(t time.Time) int {
	return int(time.Until(t).Hours() / 24)
}

// FormatExpire 友好的到期展示。
func FormatExpire(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}
