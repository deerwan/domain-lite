// Package notify 通过 Webhook 发送文本通知（飞书 / 企业微信 / Telegram / 通用）。
// 不引入额外依赖，仅使用标准库 net/http。
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config 通知渠道配置。
type Config struct {
	Enabled       bool
	Type          string // feishu | wecom | telegram | generic
	WebhookURL    string
	ChatID        string // telegram 需要
	ThresholdDays int
}

// Send 将标题与多行内容拼接为纯文本，按渠道格式 POST 到 Webhook。
// 未启用或 URL 为空时直接返回 nil（静默跳过）。
func Send(cfg Config, title string, lines []string) error {
	if !cfg.Enabled || cfg.WebhookURL == "" {
		return nil
	}
	text := title + "\n" + strings.Join(lines, "\n")

	var (
		url     = cfg.WebhookURL
		payload []byte
		err     error
	)
	switch cfg.Type {
	case "telegram":
		payload, err = json.Marshal(map[string]string{"chat_id": cfg.ChatID, "text": text})
	case "wecom":
		payload, err = json.Marshal(map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": text},
		})
	case "feishu":
		payload, err = json.Marshal(map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": text},
		})
	default: // generic
		payload, err = json.Marshal(map[string]string{"text": text})
	}
	if err != nil {
		return fmt.Errorf("notify marshal: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("notify post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify webhook status %d", resp.StatusCode)
	}
	// 飞书/企业微信即便业务失败也返回 200，需解析响应体中的业务码。
	respBody, _ := io.ReadAll(resp.Body)
	if cfg.Type == "feishu" || cfg.Type == "wecom" {
		var r struct {
			Code    int    `json:"code"`
			ErrCode int    `json:"errcode"`
			Msg     string `json:"msg"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &r) == nil {
			if cfg.Type == "feishu" && r.Code != 0 {
				return fmt.Errorf("feishu 业务错误 code=%d msg=%s", r.Code, r.Msg)
			}
			if cfg.Type == "wecom" && r.ErrCode != 0 {
				return fmt.Errorf("wecom 业务错误 errcode=%d msg=%s", r.ErrCode, r.Message)
			}
		}
	}
	return nil
}
