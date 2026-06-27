package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestWebhookServiceBuildsFeishuTextPayload(t *testing.T) {
	svc := NewWebhookService(&config.Config{Webhook: config.WebhookConfig{Format: "feishu"}})
	payload, err := svc.buildPayloadWithConfig(config.WebhookConfig{Format: "feishu"}, WebhookEvent{
		Event:     WebhookEventRedeemUsed,
		Title:     "Webhook 测试",
		Severity:  "info",
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Data: map[string]any{
			"message": "hello",
		},
	})
	if err != nil {
		t.Fatalf("build payload: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if body["msg_type"] != "text" {
		t.Fatalf("msg_type = %v, want text; payload=%s", body["msg_type"], payload)
	}
	content, ok := body["content"].(map[string]any)
	if !ok {
		t.Fatalf("content missing or invalid: %#v", body["content"])
	}
	text, _ := content["text"].(string)
	if !strings.Contains(text, "redeem.used") || !strings.Contains(text, "hello") {
		t.Fatalf("text payload missing event/data: %q", text)
	}
}

func TestWebhookServiceReturnsFeishuBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":19024,"msg":"bot disabled"}`))
	}))
	defer server.Close()

	svc := NewWebhookService(&config.Config{Webhook: config.WebhookConfig{
		Enabled:        true,
		URL:            server.URL,
		Format:         "feishu",
		TimeoutSeconds: 5,
		Events:         []string{WebhookEventRedeemUsed},
	}})
	err := svc.Notify(context.Background(), WebhookEvent{
		Event:     WebhookEventRedeemUsed,
		Title:     "Webhook 测试",
		Timestamp: time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "19024") {
		t.Fatalf("expected feishu business error, got %v", err)
	}
}

func TestWebhookServiceNotifyRequiresEnabledConfig(t *testing.T) {
	svc := NewWebhookService(&config.Config{Webhook: config.WebhookConfig{Events: []string{WebhookEventRedeemUsed}}})
	err := svc.Notify(context.Background(), WebhookEvent{Event: WebhookEventRedeemUsed, Title: "test"})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled error, got %v", err)
	}
}
