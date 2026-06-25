package handler

import (
	"bytes"
	"context"
	"io"
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
	return contentType == "" || strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/event-stream")
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
	if maxBytes <= 0 || len(body) <= maxBytes {
		return string(body), false, len(body)
	}
	return string(body[:maxBytes]), true, len(body)
}
