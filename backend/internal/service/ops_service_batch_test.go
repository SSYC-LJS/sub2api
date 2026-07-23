package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SSYC-LJS/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpsServiceRecordErrorBatch_SanitizesAndBatches(t *testing.T) {
	t.Parallel()

	var captured []*OpsInsertErrorLogInput
	repo := &opsRepoMock{
		BatchInsertErrorLogsFn: func(ctx context.Context, inputs []*OpsInsertErrorLogInput) (int64, error) {
			captured = append(captured, inputs...)
			return int64(len(inputs)), nil
		},
	}
	svc := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	msg := " upstream failed: https://example.com?access_token=secret-value "
	detail := `{"authorization":"Bearer secret-token"}`
	entries := []*OpsInsertErrorLogInput{
		{
			ErrorBody:            `{"error":"bad","access_token":"secret"}`,
			UpstreamStatusCode:   intPtr(-10),
			UpstreamErrorMessage: strPtr(msg),
			UpstreamErrorDetail:  strPtr(detail),
			UpstreamErrors: []*OpsUpstreamErrorEvent{
				{
					AccountID:          -2,
					UpstreamStatusCode: 429,
					Message:            " token leaked ",
					Detail:             `{"refresh_token":"secret"}`,
				},
			},
		},
		{
			ErrorPhase: "upstream",
			ErrorType:  "upstream_error",
			CreatedAt:  time.Now().UTC(),
		},
	}

	require.NoError(t, svc.RecordErrorBatch(context.Background(), entries))
	require.Len(t, captured, 2)

	first := captured[0]
	require.Equal(t, "internal", first.ErrorPhase)
	require.Equal(t, "api_error", first.ErrorType)
	require.Nil(t, first.UpstreamStatusCode)
	require.NotNil(t, first.UpstreamErrorMessage)
	require.NotContains(t, *first.UpstreamErrorMessage, "secret-value")
	require.Contains(t, *first.UpstreamErrorMessage, "access_token=***")
	require.NotNil(t, first.UpstreamErrorDetail)
	require.NotContains(t, *first.UpstreamErrorDetail, "secret-token")
	require.NotContains(t, first.ErrorBody, "secret")
	require.Nil(t, first.UpstreamErrors)
	require.NotNil(t, first.UpstreamErrorsJSON)
	require.NotContains(t, *first.UpstreamErrorsJSON, "secret")
	require.Contains(t, *first.UpstreamErrorsJSON, "[REDACTED]")

	second := captured[1]
	require.Equal(t, "upstream", second.ErrorPhase)
	require.Equal(t, "upstream_error", second.ErrorType)
	require.False(t, second.CreatedAt.IsZero())
}

func TestOpsServiceRecordErrorBatch_FallsBackToSingleInsert(t *testing.T) {
	t.Parallel()

	var (
		batchCalls  int
		singleCalls int
	)
	repo := &opsRepoMock{
		BatchInsertErrorLogsFn: func(ctx context.Context, inputs []*OpsInsertErrorLogInput) (int64, error) {
			batchCalls++
			return 0, errors.New("batch failed")
		},
		InsertErrorLogFn: func(ctx context.Context, input *OpsInsertErrorLogInput) (int64, error) {
			singleCalls++
			return int64(singleCalls), nil
		},
	}
	svc := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	err := svc.RecordErrorBatch(context.Background(), []*OpsInsertErrorLogInput{
		{ErrorMessage: "first"},
		{ErrorMessage: "second"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, batchCalls)
	require.Equal(t, 2, singleCalls)
}

func TestOpsServiceNotifyOpsErrorUsesEmailAndGroupName(t *testing.T) {
	requests := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		content, _ := body["content"].(map[string]any)
		if text, _ := content["text"].(string); text != "" {
			requests <- text
			return
		}
		card, _ := body["card"].(map[string]any)
		elements, _ := card["elements"].([]any)
		var parts []string
		for _, element := range elements {
			elementMap, _ := element.(map[string]any)
			textMap, _ := elementMap["text"].(map[string]any)
			if text, _ := textMap["content"].(string); text != "" {
				parts = append(parts, text)
			}
		}
		requests <- strings.Join(parts, "\n")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer server.Close()

	webhook := NewWebhookService(&config.Config{Webhook: config.WebhookConfig{
		Enabled:        true,
		URL:            server.URL,
		Format:         "feishu",
		TimeoutSeconds: 5,
		Events:         []string{WebhookEventOpsError},
	}})
	repo := &opsRepoMock{InsertErrorLogFn: func(ctx context.Context, input *OpsInsertErrorLogInput) (int64, error) {
		return 1, nil
	}}
	svc := NewOpsService(repo, nil, nil, nil,
		&opsWebhookUserRepoStub{user: &User{ID: 42, Email: "owner@example.com"}},
		&opsWebhookGroupRepoStub{group: &Group{ID: 7, Name: "Claude 低倍率分组"}},
		nil, nil, nil, nil, nil, nil,
	)
	svc.SetWebhookService(webhook)

	userID := int64(42)
	groupID := int64(7)
	err := svc.RecordError(context.Background(), &OpsInsertErrorLogInput{
		UserID:       &userID,
		GroupID:      &groupID,
		StatusCode:   429,
		ErrorMessage: "rate limited",
		ErrorPhase:   "upstream",
		ErrorType:    "account_error",
		CreatedAt:    time.Now(),
	})
	require.NoError(t, err)

	select {
	case text := <-requests:
		require.Contains(t, text, "owner@example.com")
		require.Contains(t, text, "Claude 低倍率分组")
		require.NotContains(t, text, "用户ID：42")
		require.NotContains(t, text, "报错分组：分组ID 7")
	case <-time.After(time.Second):
		t.Fatal("webhook request not received")
	}
}

type opsWebhookUserRepoStub struct {
	UserRepository
	user *User
}

func (s *opsWebhookUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	if s.user == nil || s.user.ID != id {
		return nil, ErrUserNotFound
	}
	return s.user, nil
}

type opsWebhookGroupRepoStub struct {
	GroupRepository
	group *Group
}

func (s *opsWebhookGroupRepoStub) GetByIDLite(ctx context.Context, id int64) (*Group, error) {
	if s.group == nil || s.group.ID != id {
		return nil, ErrGroupNotFound
	}
	return s.group, nil
}

func TestOpsServiceRecordErrorPersistsExplicitAccountAuthStatusZero(t *testing.T) {
	t.Parallel()

	var captured *OpsInsertErrorLogInput
	repo := &opsRepoMock{
		InsertErrorLogFn: func(_ context.Context, input *OpsInsertErrorLogInput) (int64, error) {
			captured = input
			return 1, nil
		},
	}
	svc := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	staleStatus := 403
	staleMessage := "stale inference message"
	staleDetail := "stale inference detail"

	err := svc.RecordError(context.Background(), &OpsInsertErrorLogInput{
		ErrorPhase:           "upstream",
		ErrorType:            "upstream_error",
		ErrorOwner:           "provider",
		ErrorSource:          "upstream_http",
		UpstreamStatusCode:   &staleStatus,
		UpstreamErrorMessage: &staleMessage,
		UpstreamErrorDetail:  &staleDetail,
		UpstreamErrors: []*OpsUpstreamErrorEvent{
			{Stage: string(GatewayFailureStageInference), UpstreamStatusCode: 403, Message: staleMessage, Detail: staleDetail},
			{
				Stage: string(GatewayFailureStageAccountAuth), Scope: string(GatewayFailureScopeAccount),
				Reason: string(GrokCredentialReasonRevoked), Message: "Grok OAuth credentials require account action",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, captured)
	require.Equal(t, "account_auth", captured.ErrorPhase)
	require.Equal(t, "provider", captured.ErrorOwner)
	require.Equal(t, "gateway", captured.ErrorSource)
	require.NotNil(t, captured.UpstreamStatusCode)
	require.Zero(t, *captured.UpstreamStatusCode)
	require.NotNil(t, captured.UpstreamErrorMessage)
	require.Equal(t, "Grok OAuth credentials require account action", *captured.UpstreamErrorMessage)
	require.Nil(t, captured.UpstreamErrorDetail)
	require.Nil(t, captured.UpstreamErrors)
	require.NotNil(t, captured.UpstreamErrorsJSON)
	require.Contains(t, *captured.UpstreamErrorsJSON, `"upstream_status_code":403`)
	require.Contains(t, *captured.UpstreamErrorsJSON, `"stage":"account_auth"`)
}

func strPtr(v string) *string {
	return &v
}
