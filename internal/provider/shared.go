package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// encodeID 把若干字段拼接后用 URL-safe base64 编码，作为无原生 ID 的记录标识
// （GoDaddy / Spaceship 的记录没有稳定 ID，用 type|name|content 合成）。
func encodeID(parts ...string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, "|")))
}

// decodeID 还原 encodeID 编码的记录标识。
func decodeID(id string) []string {
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return nil
	}
	return strings.Split(string(b), "|")
}

// splitContent 把逗号分隔的多值记录值拆成数组（GoDaddy data 字段用）。
func splitContent(s string) []string {
	if s == "" {
		return []string{""}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// ttlOf 返回合法 TTL，缺省回退到 600。
func ttlOf(ttl int) int {
	if ttl <= 0 {
		return 600
	}
	return ttl
}

// statusErr 把非 2xx 响应转成易读错误。
func statusErr(prefix string, resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%s: %s: %s", prefix, resp.Status, string(b))
}

// extractExt 从账户 Ext(JSON 字符串) 中取某个 key。
func extractExt(ext, key string) string {
	if ext == "" {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(ext), &m); err != nil {
		return ""
	}
	return m[key]
}
