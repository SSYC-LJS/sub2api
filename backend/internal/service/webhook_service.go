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
	cfg        config.WebhookConfig
	httpClient *http.Client
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

func (s *WebhookService) NotifyAsync(event WebhookEvent) {
	if s == nil || !s.cfg.Enabled || strings.TrimSpace(s.cfg.URL) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout())
		defer cancel()
		if err := s.Notify(ctx, event); err != nil {
			log.Printf("[Webhook] notify failed: %v", err)
		}
	}()
}

func (s *WebhookService) Notify(ctx context.Context, event WebhookEvent) error {
	if s == nil || !s.cfg.Enabled || strings.TrimSpace(s.cfg.URL) == "" {
		return nil
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	payload, err := s.buildPayload(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(s.cfg.URL), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(s.cfg.BearerToken); token != "" {
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
	format := strings.ToLower(strings.TrimSpace(s.cfg.Format))
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
