package whois

import (
	"context"
	"io"
	"net"
	"regexp"
	"strings"
	"time"
)

// Info 解析后的 WHOIS 核心信息。
type Info struct {
	Registrar string
	ExpireAt  *time.Time
	Status    string
}

var (
	reReferral  = regexp.MustCompile(`(?i)whois:\s*(\S+)`)
	reExpiry    = regexp.MustCompile(`(?im)(?:Registrar Registration Expiration Date|Registry Expiry Date|Expiry Date|Expiration Date|Expires On|Expires):\s*(.+)`)
	reRegistrar = regexp.MustCompile(`(?im)(?:Sponsoring Registrar|Registrar):\s*(.+)`)
	reStatus    = regexp.MustCompile(`(?im)Domain Status:\s*(\S+)`)
	reTrailing  = regexp.MustCompile(`\s*\([^)]*\)\s*$`)
)

// Lookup 查询域名的 WHOIS 信息：先向 IANA 获取注册局 WHOIS 服务器，
// 再向注册局查询并解析。整个过程受 ctx 超时约束；失败返回 error，由调用方降级处理。
func Lookup(ctx context.Context, domain string) (*Info, error) {
	server, err := referralServer(ctx, domain)
	if err != nil {
		return nil, err
	}
	raw, err := query(ctx, server, domain)
	if err != nil {
		return nil, err
	}
	return parse(raw), nil
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
		info.Registrar = strings.TrimSpace(m[1])
	}
	if m := reStatus.FindStringSubmatch(raw); len(m) >= 2 {
		info.Status = strings.TrimSpace(m[1])
	}
	return info
}

func parseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	s = reTrailing.ReplaceAllString(s, "")
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"02-Jan-2006",
		"02 Jan 2006",
		"January 2 2006",
		"January 02, 2006",
		"2006/01/02",
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
