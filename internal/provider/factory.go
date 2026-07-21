package provider

import (
	"fmt"

	"domain-lite/internal/config"
	"domain-lite/internal/crypto"
	"domain-lite/internal/model"
)

// New 根据账户类型与凭据构造对应 provider。SecretEnc 在内部解密。
// Cloudflare: secret_key = API Token（仅 Bearer，不需要 email）。
// 阿里云: access_key = AccessKeyId, secret_key = AccessKeySecret。
func New(cfg *config.Config, acc *model.DnsAccount) (DnsProvider, error) {
	secret, err := crypto.Decrypt(acc.SecretEnc, cfg.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	switch acc.Type {
	case "cloudflare":
		return NewCloudflare(secret), nil
	case "aliyun":
		return NewAliyun(acc.AccessKey, secret), nil
	case "godaddy":
		return newGodaddy(acc.AccessKey, secret), nil
	case "dnspod":
		return newDNSPod(secret), nil
	case "namecheap":
		ip := extractExt(acc.Ext, "client_ip")
		return newNamecheap(acc.AccessKey, secret, acc.AccessKey, ip), nil
	case "spaceship":
		return newSpaceship(acc.AccessKey, secret), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", acc.Type)
	}
}
