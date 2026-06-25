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

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const defaultCaptureBodyMaxBytes = 64 * 1024

type captureResponseWriter struct {
	gin.ResponseWriter
	buf      bytes.Buffer
	maxBytes int
	total    int
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	w.capture(data)
	return w.ResponseWriter.Write(data)
}

func (w *captureResponseWriter) WriteString(data string) (int, error) {
	w.capture([]byte(data))
	return w.ResponseWriter.WriteString(data)
}

func (w *captureResponseWriter) capture(data []byte) {
	if len(data) == 0 {
		return
	}
	w.total += len(data)
	remaining := w.maxBytes - w.buf.Len()
	if remaining <= 0 {
		return
	}
	if len(data) > remaining {
		data = data[:remaining]
	}
	_, _ = w.buf.Write(data)
}

func (w *captureResponseWriter) captured() (string, bool, int) {
	return w.buf.String(), w.total > w.buf.Len(), w.total
}

func (h *GatewayHandler) RequestResponseCaptureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.requestResponseCaptureService == nil || h.cfg == nil {
			c.Next()
			return
		}
		captureSettings := h.requestResponseCaptureService.Settings(c.Request.Context())
		if !captureSettings.Enabled {
			c.Next()
			return
		}
		if !shouldCaptureGatewayBody(c.Request) {
			c.Next()
			return
		}

		apiKey, ok := middleware2.GetAPIKeyFromContext(c)
		if !ok {
			c.Next()
			return
		}
		subject, ok := middleware2.GetAuthSubjectFromContext(c)
		if !ok {
			c.Next()
			return
		}

		maxBytes := captureSettings.MaxBodyBytes
		if maxBytes <= 0 {
			maxBytes = defaultCaptureBodyMaxBytes
		}
		if maxBytes > int(h.cfg.Gateway.MaxBodySize) && h.cfg.Gateway.MaxBodySize > 0 {
			maxBytes = int(h.cfg.Gateway.MaxBodySize)
		}

		requestBody, requestTruncated, requestBytes := captureRequestBody(c.Request, maxBytes)
		captureWriter := &captureResponseWriter{ResponseWriter: c.Writer, maxBytes: maxBytes}
		c.Writer = captureWriter
		startedAt := time.Now()

		c.Next()

		// Restore c.Writer before post-processing so outer middlewares
		// (opsErrorLogger, Logger, Recovery) never see our wrapper.
		c.Writer = captureWriter.ResponseWriter

		responseBody, responseTruncated, responseBytes := captureWriter.captured()
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

func shouldCaptureGatewayBody(r *http.Request) bool {
	if r == nil || r.Body == nil {
		return false
	}
	method := strings.ToUpper(r.Method)
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
		return false
	}
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	return contentType == "" || strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/event-stream") || strings.Contains(contentType, "multipart/form-data")
}

func captureRequestBody(r *http.Request, maxBytes int) (string, bool, int) {
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
		return summarizeMultipartRequestBody(r.Header.Get("Content-Type"), body, maxBytes)
	}
	if maxBytes <= 0 || len(body) <= maxBytes {
		return string(body), false, len(body)
	}
	return string(body[:maxBytes]), true, len(body)
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

func summarizeMultipartRequestBody(contentType string, body []byte, maxBytes int) (string, bool, int) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return "", false, len(body)
	}

	summary := multipartCaptureSummary{Multipart: true, Fields: make(map[string][]string)}
	remainingImageBytes := maxBytes
	if remainingImageBytes <= 0 || remainingImageBytes > defaultCaptureBodyMaxBytes {
		remainingImageBytes = defaultCaptureBodyMaxBytes
	}
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
			fileSummary, consumed := summarizeMultipartFilePart(part, name, filename, remainingImageBytes)
			remainingImageBytes -= consumed
			if remainingImageBytes < 0 {
				remainingImageBytes = 0
			}
			summary.Files = append(summary.Files, fileSummary)
			_ = part.Close()
			continue
		}
		value, _ := io.ReadAll(io.LimitReader(part, int64(defaultCaptureBodyMaxBytes)+1))
		fieldValue := string(value)
		summary.Fields[name] = append(summary.Fields[name], fieldValue)
		if strings.EqualFold(name, "model") && summary.Model == "" {
			summary.Model = fieldValue
		}
		_ = part.Close()
	}

	encoded, err := json.Marshal(summary)
	if err != nil {
		return "", false, len(body)
	}
	if maxBytes <= 0 || len(encoded) <= maxBytes {
		return string(encoded), false, len(body)
	}
	return string(encoded[:maxBytes]), true, len(body)
}

func summarizeMultipartFilePart(part *multipart.Part, field, filename string, maxPreviewBytes int) (multipartCaptureFileSummary, int) {
	contentType := strings.TrimSpace(part.Header.Get("Content-Type"))
	out := multipartCaptureFileSummary{Field: field, Filename: filename, ContentType: contentType}
	isImage := isImageContentType(contentType)
	if !isImage || maxPreviewBytes <= 0 {
		size, _ := io.Copy(io.Discard, part)
		out.Size = size
		out.Truncated = isImage && size > 0
		return out, 0
	}

	preview, _ := io.ReadAll(io.LimitReader(part, int64(maxPreviewBytes)+1))
	if len(preview) > maxPreviewBytes {
		out.Truncated = true
		preview = preview[:maxPreviewBytes]
	}
	remainingSize, _ := io.Copy(io.Discard, part)
	out.Size = int64(len(preview)) + remainingSize
	if len(preview) > 0 {
		out.DataURL = "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(preview)
	}
	return out, len(preview)
}

func isImageContentType(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "image/")
}
