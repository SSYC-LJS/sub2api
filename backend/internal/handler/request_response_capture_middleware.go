package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

type captureResponseWriter struct {
	gin.ResponseWriter
	buf   bytes.Buffer
	total int64
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.capture(data[:n])
	return n, err
}

func (w *captureResponseWriter) WriteString(data string) (int, error) {
	n, err := w.ResponseWriter.WriteString(data)
	w.capture([]byte(data[:n]))
	return n, err
}

func (w *captureResponseWriter) capture(data []byte) {
	if len(data) == 0 {
		return
	}
	w.total += int64(len(data))
	_, _ = w.buf.Write(data)
}

func (w *captureResponseWriter) captured(contentType string) (string, bool, int64) {
	return encodeCapturedBody(contentType, w.buf.Bytes()), false, w.total
}

func (h *GatewayHandler) RequestResponseCaptureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.requestResponseCaptureService == nil {
			c.Next()
			return
		}
		captureSettings := h.requestResponseCaptureService.Settings(c.Request.Context())
		if !captureSettings.Enabled {
			c.Next()
			return
		}
		apiKey, ok := middleware2.GetAPIKeyFromContext(c)
		if !ok {
			c.Next()
			return
		}
		if !captureSettings.CapturesGroup(apiKey.GroupID) {
			c.Next()
			return
		}
		subject, ok := middleware2.GetAuthSubjectFromContext(c)
		if !ok {
			c.Next()
			return
		}

		requestBody, requestTruncated, requestBytes := captureRequestBody(c.Request)
		captureWriter := &captureResponseWriter{ResponseWriter: c.Writer}
		c.Writer = captureWriter
		startedAt := time.Now()

		c.Next()

		// Restore c.Writer before post-processing so outer middlewares
		// (opsErrorLogger, Logger, Recovery) never see our wrapper.
		c.Writer = captureWriter.ResponseWriter

		responseBody, responseTruncated, responseBytes := captureWriter.captured(captureWriter.Header().Get("Content-Type"))
		endpoint := GetInboundEndpoint(c)
		if endpoint == "" {
			endpoint = c.FullPath()
		}
		model := gjson.Get(requestBody, "model").String()
		reqStream, _ := parseOpenAICompatibleStream([]byte(requestBody))
		statusCode := captureWriter.Status()
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		durationMs := int(time.Since(startedAt).Milliseconds())

		logEntry := &service.RequestResponseLog{
			RequestID:         c.GetHeader("X-Request-ID"),
			UserID:            subject.UserID,
			APIKeyID:          apiKey.ID,
			GroupID:           apiKey.GroupID,
			Method:            c.Request.Method,
			Path:              c.Request.URL.Path,
			Endpoint:          endpoint,
			Model:             model,
			Stream:            reqStream,
			StatusCode:        statusCode,
			RequestBody:       requestBody,
			ResponseBody:      responseBody,
			RequestTruncated:  requestTruncated,
			ResponseTruncated: responseTruncated,
			RequestBodyBytes:  requestBytes,
			ResponseBodyBytes: responseBytes,
			DurationMs:        int64(durationMs),
			UserAgent:         c.GetHeader("User-Agent"),
			IPAddress:         ip.GetClientIP(c),
			CreatedAt:         startedAt,
		}
		reqLog := requestLogger(c, "handler.gateway.request_response_capture")
		h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
			if err := h.requestResponseCaptureService.Create(ctx, logEntry); err != nil {
				reqLog.Warn("request_response_capture.create_failed", zap.Error(err))
			}
		})
	}
}

func captureRequestBody(r *http.Request) (string, bool, int64) {
	if r == nil || r.Body == nil {
		return "", false, 0
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(nil))
		return "", false, 0
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if isMultipartFormContentType(r.Header.Get("Content-Type")) {
		return summarizeMultipartRequestBody(r.Header.Get("Content-Type"), body)
	}
	return encodeCapturedBody(r.Header.Get("Content-Type"), body), false, int64(len(body))
}

type multipartCaptureSummary struct {
	Multipart bool                          `json:"multipart"`
	Model     string                        `json:"model,omitempty"`
	Fields    map[string][]string           `json:"fields,omitempty"`
	Files     []multipartCaptureFileSummary `json:"files,omitempty"`
}

type multipartCaptureFileSummary struct {
	Field       string `json:"field"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size"`
	DataURL     string `json:"data_url,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
}

func isMultipartFormContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "multipart/form-data")
}

func summarizeMultipartRequestBody(contentType string, body []byte) (string, bool, int64) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return encodeCapturedBody(contentType, body), false, int64(len(body))
	}

	summary := multipartCaptureSummary{Multipart: true, Fields: make(map[string][]string)}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		name := strings.TrimSpace(part.FormName())
		if name == "" {
			_ = part.Close()
			continue
		}
		filename := strings.TrimSpace(part.FileName())
		if filename != "" {
			summary.Files = append(summary.Files, summarizeMultipartFilePart(part, name, filename))
			_ = part.Close()
			continue
		}
		value, _ := io.ReadAll(part)
		fieldValue := string(value)
		summary.Fields[name] = append(summary.Fields[name], fieldValue)
		if strings.EqualFold(name, "model") && summary.Model == "" {
			summary.Model = fieldValue
		}
		_ = part.Close()
	}

	encoded, err := json.Marshal(summary)
	if err != nil {
		return encodeCapturedBody(contentType, body), false, int64(len(body))
	}
	return string(encoded), false, int64(len(body))
}

func summarizeMultipartFilePart(part *multipart.Part, field, filename string) multipartCaptureFileSummary {
	contentType := strings.TrimSpace(part.Header.Get("Content-Type"))
	out := multipartCaptureFileSummary{Field: field, Filename: filename, ContentType: contentType}
	data, _ := io.ReadAll(part)
	out.Size = int64(len(data))
	if len(data) > 0 {
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		out.DataURL = "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return out
}

type binaryCaptureEnvelope struct {
	Encoding    string `json:"capture_encoding"`
	ContentType string `json:"content_type,omitempty"`
	Data        string `json:"data"`
}

func encodeCapturedBody(contentType string, body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if utf8.Valid(body) {
		return string(body)
	}
	encoded, err := json.Marshal(binaryCaptureEnvelope{
		Encoding:    "base64",
		ContentType: strings.TrimSpace(contentType),
		Data:        base64.StdEncoding.EncodeToString(body),
	})
	if err != nil {
		return ""
	}
	return string(encoded)
}
