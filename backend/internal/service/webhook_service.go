package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const defaultWebhookTimeout = 5 * time.Second

type WebhookService struct {
	cfg         config.WebhookConfig
	settingRepo SettingRepository
	httpClient  *http.Client
}

type WebhookEvent struct {
	Event     string         `json:"event"`
	Title     string         `json:"title"`
	Severity  string         `json:"severity,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

func NewWebhookService(cfg *config.Config) *WebhookService {
	if cfg == nil {
		return &WebhookService{httpClient: &http.Client{Timeout: defaultWebhookTimeout}}
	}
	timeout := time.Duration(cfg.Webhook.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultWebhookTimeout
	}
	return &WebhookService{
		cfg:        cfg.Webhook,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (s *WebhookService) SetSettingRepository(repo SettingRepository) {
	if s == nil {
		return
	}
	s.settingRepo = repo
}

func (s *WebhookService) effectiveConfig(ctx context.Context) config.WebhookConfig {
	cfg := s.cfg
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = int(defaultWebhookTimeout / time.Second)
	}
	if s == nil || s.settingRepo == nil {
		return cfg
	}
	settings, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyWebhookEnabled,
		SettingKeyWebhookURL,
		SettingKeyWebhookFormat,
		SettingKeyWebhookBearerToken,
		SettingKeyWebhookTimeoutSeconds,
	})
	if err != nil {
		return cfg
	}
	if raw, ok := settings[SettingKeyWebhookEnabled]; ok {
		cfg.Enabled = strings.TrimSpace(raw) == "true"
	}
	if raw, ok := settings[SettingKeyWebhookURL]; ok {
		cfg.URL = strings.TrimSpace(raw)
	}
	if raw, ok := settings[SettingKeyWebhookFormat]; ok {
		format := strings.ToLower(strings.TrimSpace(raw))
		if format == "json" {
			cfg.Format = "json"
		} else if format != "" {
			cfg.Format = "feishu"
		}
	}
	if raw, ok := settings[SettingKeyWebhookBearerToken]; ok && strings.TrimSpace(raw) != "" {
		cfg.BearerToken = strings.TrimSpace(raw)
	}
	if raw, ok := settings[SettingKeyWebhookTimeoutSeconds]; ok {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &n); err == nil && n > 0 {
			cfg.TimeoutSeconds = n
		}
	}
	if cfg.TimeoutSeconds < 1 {
		cfg.TimeoutSeconds = int(defaultWebhookTimeout / time.Second)
	}
	if cfg.TimeoutSeconds > 30 {
		cfg.TimeoutSeconds = 30
	}
	return cfg
}

func (s *WebhookService) NotifyAsync(event WebhookEvent) {
	if s == nil {
		return
	}
	go func() {
		cfg := s.effectiveConfig(context.Background())
		if !cfg.Enabled || strings.TrimSpace(cfg.URL) == "" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
		defer cancel()
		if err := s.notifyWithConfig(ctx, cfg, event); err != nil {
			log.Printf("[Webhook] notify failed: %v", err)
		}
	}()
}

func (s *WebhookService) Notify(ctx context.Context, event WebhookEvent) error {
	if s == nil {
		return nil
	}
	cfg := s.effectiveConfig(ctx)
	return s.notifyWithConfig(ctx, cfg, event)
}

func (s *WebhookService) notifyWithConfig(ctx context.Context, cfg config.WebhookConfig, event WebhookEvent) error {
	if s == nil || !cfg.Enabled || strings.TrimSpace(cfg.URL) == "" {
		return nil
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	payload, err := s.buildPayloadWithConfig(cfg, event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(cfg.URL), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(cfg.BearerToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *WebhookService) buildPayload(event WebhookEvent) ([]byte, error) {
	return s.buildPayloadWithConfig(s.cfg, event)
}

func (s *WebhookService) buildPayloadWithConfig(cfg config.WebhookConfig, event WebhookEvent) ([]byte, error) {
	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	if format == "" || format == "feishu" || format == "lark" {
		return json.Marshal(map[string]any{
			"msg_type": "interactive",
			"card": map[string]any{
				"schema": "2.0",
				"config": map[string]any{"wide_screen_mode": true},
				"header": map[string]any{
					"title":    map[string]any{"tag": "plain_text", "content": event.Title},
					"template": feishuTemplate(event.Severity),
				},
				"body": map[string]any{
					"elements": []any{map[string]any{
						"tag":     "markdown",
						"content": feishuMarkdown(event),
					}},
				},
			},
		})
	}
	return json.Marshal(event)
}

func (s *WebhookService) timeout() time.Duration {
	if s == nil || s.httpClient == nil || s.httpClient.Timeout <= 0 {
		return defaultWebhookTimeout
	}
	return s.httpClient.Timeout
}

func feishuTemplate(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "error", "critical":
		return "red"
	case "warning", "warn":
		return "orange"
	case "success":
		return "green"
	default:
		return "blue"
	}
}

func feishuMarkdown(event WebhookEvent) string {
	var b strings.Builder
	b.WriteString("**事件**：")
	b.WriteString(escapeFeishuText(event.Event))
	b.WriteString("\n**时间**：")
	b.WriteString(event.Timestamp.Format(time.RFC3339))
	if event.Severity != "" {
		b.WriteString("\n**级别**：")
		b.WriteString(escapeFeishuText(event.Severity))
	}
	for _, key := range sortedWebhookKeys(event.Data) {
		b.WriteString("\n**")
		b.WriteString(escapeFeishuText(key))
		b.WriteString("**：")
		b.WriteString(escapeFeishuText(fmt.Sprint(event.Data[key])))
	}
	return b.String()
}

func sortedWebhookKeys(data map[string]any) []string {
	preferred := []string{"user_id", "user_email", "group_id", "group_name", "account_id", "account_name", "platform", "model", "status_code", "message", "code", "type", "value", "validity_days"}
	keys := make([]string, 0, len(data))
	seen := map[string]bool{}
	for _, key := range preferred {
		if _, ok := data[key]; ok {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	for key := range data {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	return keys
}

func escapeFeishuText(s string) string {
	s = strings.ReplaceAll(s, "<", "＜")
	s = strings.ReplaceAll(s, ">", "＞")
	return s
}
