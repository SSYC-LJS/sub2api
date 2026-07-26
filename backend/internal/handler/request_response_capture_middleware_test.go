package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type requestResponseLogRepoStub struct {
	logs []service.RequestResponseLog
}

func (r *requestResponseLogRepoStub) Create(_ context.Context, log *service.RequestResponseLog) error {
	r.logs = append(r.logs, *log)
	return nil
}

func (r *requestResponseLogRepoStub) List(context.Context, int, int, service.RequestResponseLogFilters) ([]service.RequestResponseLog, int64, error) {
	return nil, 0, nil
}

func (r *requestResponseLogRepoStub) ListForExport(context.Context, service.RequestResponseLogFilters, int) ([]service.RequestResponseLog, error) {
	return nil, nil
}

func (r *requestResponseLogRepoStub) GetByID(context.Context, int64) (*service.RequestResponseLog, error) {
	return nil, service.ErrSettingNotFound
}

type requestResponseSettingsReaderStub struct {
	settings service.RequestResponseCaptureSettings
}

func (r requestResponseSettingsReaderStub) GetRequestResponseCaptureSettings(context.Context) service.RequestResponseCaptureSettings {
	return r.settings
}

func TestCaptureRequestBody_MultipartPreservesCompleteFileContent(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("model", "gpt-image-1"))
	require.NoError(t, mw.WriteField("prompt", "make a cat"))
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="cat.png"`)
	h.Set("Content-Type", "image/png")
	fw, err := mw.CreatePart(h)
	require.NoError(t, err)
	_, err = fw.Write([]byte("PNG_BINARY_SHOULD_NOT_BE_SAVED"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	body, truncated, bodyBytes := captureRequestBody(req)
	require.False(t, truncated)
	require.Equal(t, int64(buf.Len()), bodyBytes)
	require.True(t, gjson.Get(body, "multipart").Bool())
	require.Equal(t, "gpt-image-1", gjson.Get(body, "model").String())
	require.Equal(t, "make a cat", gjson.Get(body, "fields.prompt.0").String())
	require.Equal(t, "image", gjson.Get(body, "files.0.field").String())
	require.Equal(t, "cat.png", gjson.Get(body, "files.0.filename").String())
	require.Equal(t, "image/png", gjson.Get(body, "files.0.content_type").String())
	require.Equal(t, int64(len("PNG_BINARY_SHOULD_NOT_BE_SAVED")), gjson.Get(body, "files.0.size").Int())
	dataURL := gjson.Get(body, "files.0.data_url").String()
	require.Contains(t, dataURL, "data:image/png;base64,")
	encoded := strings.TrimPrefix(dataURL, "data:image/png;base64,")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	require.Equal(t, []byte("PNG_BINARY_SHOULD_NOT_BE_SAVED"), decoded)

	restored, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, buf.Bytes(), restored)
}

func TestCaptureRequestBody_DoesNotTruncateLargeJSON(t *testing.T) {
	payload := `{"input":"` + strings.Repeat("x", 2*1024*1024) + `"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	body, truncated, bodyBytes := captureRequestBody(req)

	require.False(t, truncated)
	require.Equal(t, int64(len(payload)), bodyBytes)
	require.Equal(t, payload, body)
}

func TestCaptureRequestBody_EncodesBinaryWithoutDataLoss(t *testing.T) {
	payload := []byte{0xff, 0x00, 0x7f, 0x80}
	req, err := http.NewRequest(http.MethodPost, "/v1/files", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/octet-stream")

	body, truncated, bodyBytes := captureRequestBody(req)

	require.False(t, truncated)
	require.Equal(t, int64(len(payload)), bodyBytes)
	require.Equal(t, "base64", gjson.Get(body, "capture_encoding").String())
	decoded, err := base64.StdEncoding.DecodeString(gjson.Get(body, "data").String())
	require.NoError(t, err)
	require.Equal(t, payload, decoded)
}

func TestCaptureResponseWriter_DoesNotTruncateLargeResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	writer := &captureResponseWriter{ResponseWriter: ctx.Writer}
	payload := []byte(strings.Repeat("response-data", 200000))

	n, err := writer.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)

	body, truncated, bodyBytes := writer.captured("text/event-stream")
	require.False(t, truncated)
	require.Equal(t, int64(len(payload)), bodyBytes)
	require.Equal(t, string(payload), body)
	require.Equal(t, payload, recorder.Body.Bytes())
}

func TestRequestResponseCaptureMiddleware_FiltersGroupAndCapturesGETResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name        string
		apiGroupID  int64
		wantEntries int
	}{
		{name: "matching group", apiGroupID: 10, wantEntries: 1},
		{name: "different group", apiGroupID: 20, wantEntries: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &requestResponseLogRepoStub{}
			captureService := service.NewRequestResponseCaptureService(repo, nil, requestResponseSettingsReaderStub{
				settings: service.RequestResponseCaptureSettings{Enabled: true, GroupID: 10},
			})
			h := &GatewayHandler{requestResponseCaptureService: captureService}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				groupID := tt.apiGroupID
				c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 3, UserID: 5, GroupID: &groupID})
				c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 5})
				c.Next()
			})
			router.Use(h.RequestResponseCaptureMiddleware())
			router.GET("/v1/models", func(c *gin.Context) {
				c.Data(http.StatusOK, "application/json", []byte(`{"models":["one","two"]}`))
			})

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Len(t, repo.logs, tt.wantEntries)
			if tt.wantEntries == 1 {
				require.Empty(t, repo.logs[0].RequestBody)
				require.Equal(t, `{"models":["one","two"]}`, repo.logs[0].ResponseBody)
				require.False(t, repo.logs[0].ResponseTruncated)
			}
		})
	}
}
