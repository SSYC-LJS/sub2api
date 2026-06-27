package service

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestWebhookServiceUsesSettingRepositoryForBusinessEvents(t *testing.T) {
	requests := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		content, _ := body["content"].(map[string]any)
		requests <- fmt.Sprint(content["text"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer server.Close()

	svc := NewWebhookService(&config.Config{})
	svc.SetSettingRepository(&webhookSettingRepoStub{values: map[string]string{
		SettingKeyWebhookEnabled:        "true",
		SettingKeyWebhookURL:            server.URL,
		SettingKeyWebhookFormat:         "feishu",
		SettingKeyWebhookTimeoutSeconds: "5",
		SettingKeyWebhookEvents:         `["user.registered","redeem.used","ops.error"]`,
	}})

	err := svc.Notify(context.Background(), WebhookEvent{
		Event:     WebhookEventUserRegistered,
		Title:     "新用户注册",
		Timestamp: time.Date(2026, 6, 28, 1, 2, 3, 0, time.UTC),
		Data: map[string]any{
			"注册时间": "2026-06-28T01:02:03Z",
			"注册邮箱": "user@example.com",
		},
	})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	select {
	case text := <-requests:
		if !strings.Contains(text, "注册邮箱：user@example.com") || !strings.Contains(text, "注册时间：2026-06-28T01:02:03Z") {
			t.Fatalf("business event text missing registration fields: %q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("webhook request not received")
	}
}

func TestWebhookServiceBusinessPayloadFields(t *testing.T) {
	tests := []struct {
		name  string
		event WebhookEvent
		want  []string
	}{
		{
			name: "redeem used fields",
			event: WebhookEvent{Event: WebhookEventRedeemUsed, Title: "用户使用兑换码", Timestamp: time.Now(), Data: map[string]any{
				"使用用户邮箱": "redeem@example.com",
				"兑换码":    "ABC123",
				"兑换码额度":  "余额 10",
			}},
			want: []string{"使用用户邮箱：redeem@example.com", "兑换码：ABC123", "兑换码额度：余额 10"},
		},
		{
			name: "ops error fields",
			event: WebhookEvent{Event: WebhookEventOpsError, Title: "网关异常/账号报错", Timestamp: time.Now(), Data: map[string]any{
				"报错Code": 429,
				"报错内容":   "rate limited",
				"报错分组":   "分组ID 7",
			}},
			want: []string{"报错Code：429", "报错内容：rate limited", "报错分组：分组ID 7"},
		},
	}

	svc := NewWebhookService(&config.Config{Webhook: config.WebhookConfig{Format: "feishu"}})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := svc.buildPayloadWithConfig(config.WebhookConfig{Format: "feishu"}, tt.event)
			if err != nil {
				t.Fatalf("build payload: %v", err)
			}
			var body map[string]any
			if err := json.Unmarshal(payload, &body); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			content, _ := body["content"].(map[string]any)
			text := fmt.Sprint(content["text"])
			for _, want := range tt.want {
				if !strings.Contains(text, want) {
					t.Fatalf("payload missing %q: %q", want, text)
				}
			}
		})
	}
}

type webhookSettingRepoStub struct {
	values map[string]string
}

func (r *webhookSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	return &Setting{Key: key, Value: r.values[key]}, nil
}

func (r *webhookSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	return r.values[key], nil
}

func (r *webhookSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *webhookSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := map[string]string{}
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *webhookSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *webhookSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	result := map[string]string{}
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *webhookSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(r.values, key)
	return nil
}
