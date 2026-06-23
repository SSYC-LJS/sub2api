package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestRequestResponseLogRepositoryCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	groupID := int64(30)
	createdAt := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	log := &service.RequestResponseLog{
		RequestID:         "req-1",
		UserID:            10,
		APIKeyID:          20,
		GroupID:           &groupID,
		Method:            "POST",
		Path:              "/v1/chat/completions",
		Endpoint:          "/v1/chat/completions",
		Model:             "gpt-test",
		Stream:            true,
		StatusCode:        200,
		RequestBody:       `{"model":"gpt-test"}`,
		ResponseBody:      `data: {"ok":true}`,
		RequestTruncated:  false,
		ResponseTruncated: true,
		RequestBodyBytes:  20,
		ResponseBodyBytes: 100000,
		DurationMs:        1234,
		UserAgent:         "client-test",
		IPAddress:         "127.0.0.1",
		CreatedAt:         createdAt,
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO request_response_logs")).
		WithArgs(
			"req-1", int64(10), int64(20), &groupID,
			"POST", "/v1/chat/completions", "/v1/chat/completions", "gpt-test", true,
			200, `{"model":"gpt-test"}`, `data: {"ok":true}`, false, true,
			20, 100000, 1234, "client-test", "127.0.0.1", createdAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewRequestResponseLogRepository(db)
	if err := repo.Create(context.Background(), log); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRequestResponseLogRepositoryCreateEmptyStringsAsNull(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	log := &service.RequestResponseLog{
		UserID:     10,
		APIKeyID:   20,
		Method:     "POST",
		Path:       "/v1/messages",
		StatusCode: 400,
		CreatedAt:  createdAt,
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO request_response_logs")).
		WithArgs(
			nil, int64(10), int64(20), nil,
			"POST", "/v1/messages", nil, nil, false,
			400, "", "", false, false,
			0, 0, 0, nil, nil, createdAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewRequestResponseLogRepository(db)
	if err := repo.Create(context.Background(), log); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
