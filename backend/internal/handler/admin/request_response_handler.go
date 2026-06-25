package admin

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type RequestResponseHandler struct {
	captureService *service.RequestResponseCaptureService
	settingService *service.SettingService
}

func NewRequestResponseHandler(captureService *service.RequestResponseCaptureService, settingService *service.SettingService) *RequestResponseHandler {
	return &RequestResponseHandler{captureService: captureService, settingService: settingService}
}

type updateRequestResponseCaptureSettingsRequest struct {
	Enabled      bool `json:"enabled"`
	MaxBodyBytes int  `json:"max_body_bytes"`
}

func (h *RequestResponseHandler) GetSettings(c *gin.Context) {
	settings := h.settingService.GetRequestResponseCaptureSettings(c.Request.Context())
	response.Success(c, settings)
}

func (h *RequestResponseHandler) UpdateSettings(c *gin.Context) {
	var req updateRequestResponseCaptureSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	settings, err := h.settingService.UpdateRequestResponseCaptureSettings(c.Request.Context(), service.RequestResponseCaptureSettings{
		Enabled:      req.Enabled,
		MaxBodyBytes: req.MaxBodyBytes,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *RequestResponseHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filters, ok := parseRequestResponseLogFilters(c)
	if !ok {
		return
	}
	items, total, err := h.captureService.List(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *RequestResponseHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return
	}
	item, err := h.captureService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			response.NotFound(c, "request response log not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *RequestResponseHandler) Export(c *gin.Context) {
	filters, ok := parseRequestResponseLogFilters(c)
	if !ok {
		return
	}
	limit := 5000
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 10000 {
			limit = n
		}
	}
	items, err := h.captureService.ListForExport(c.Request.Context(), filters, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="request_response_logs_%s.csv"`, time.Now().Format("20060102_150405")))

	w := csv.NewWriter(c.Writer)
	header := []string{"ID", "RequestID", "Time", "UserID", "APIKeyID", "GroupID", "Method", "Path", "Endpoint", "Model", "Stream", "StatusCode", "DurationMs", "RequestBodyBytes", "ResponseBodyBytes", "RequestTruncated", "ResponseTruncated", "UserAgent", "IPAddress", "RequestBody", "ResponseBody"}
	if err := w.Write(header); err != nil {
		return
	}
	for _, item := range items {
		groupID := ""
		if item.GroupID != nil {
			groupID = strconv.FormatInt(*item.GroupID, 10)
		}
		row := []string{
			strconv.FormatInt(item.ID, 10),
			item.RequestID,
			item.CreatedAt.Format(time.RFC3339),
			strconv.FormatInt(item.UserID, 10),
			strconv.FormatInt(item.APIKeyID, 10),
			groupID,
			item.Method,
			item.Path,
			item.Endpoint,
			item.Model,
			strconv.FormatBool(item.Stream),
			strconv.Itoa(item.StatusCode),
			strconv.FormatInt(item.DurationMs, 10),
			strconv.Itoa(item.RequestBodyBytes),
			strconv.Itoa(item.ResponseBodyBytes),
			strconv.FormatBool(item.RequestTruncated),
			strconv.FormatBool(item.ResponseTruncated),
			item.UserAgent,
			item.IPAddress,
			item.RequestBody,
			item.ResponseBody,
		}
		if err := w.Write(row); err != nil {
			return
		}
	}
	w.Flush()
}

func parseRequestResponseLogFilters(c *gin.Context) (service.RequestResponseLogFilters, bool) {
	var filters service.RequestResponseLogFilters
	for _, spec := range []struct {
		name string
		dest *int64
	}{
		{"user_id", &filters.UserID},
		{"api_key_id", &filters.APIKeyID},
		{"group_id", &filters.GroupID},
	} {
		if raw := strings.TrimSpace(c.Query(spec.name)); raw != "" {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id < 0 {
				response.BadRequest(c, "invalid "+spec.name)
				return filters, false
			}
			*spec.dest = id
		}
	}
	filters.Endpoint = strings.TrimSpace(c.Query("endpoint"))
	filters.Model = strings.TrimSpace(c.Query("model"))
	filters.Path = strings.TrimSpace(c.Query("path"))
	filters.Search = strings.TrimSpace(c.Query("search"))
	userTZ := c.Query("timezone")
	if raw := strings.TrimSpace(c.Query("start_date")); raw != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ)
		if err != nil {
			response.BadRequest(c, "invalid start_date format, use YYYY-MM-DD")
			return filters, false
		}
		filters.StartTime = &t
	}
	if raw := strings.TrimSpace(c.Query("end_date")); raw != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ)
		if err != nil {
			response.BadRequest(c, "invalid end_date format, use YYYY-MM-DD")
			return filters, false
		}
		t = t.AddDate(0, 0, 1)
		filters.EndTime = &t
	}
	if raw := strings.TrimSpace(c.Query("start_time")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.BadRequest(c, "invalid start_time format, use RFC3339")
			return filters, false
		}
		filters.StartTime = &t
	}
	if raw := strings.TrimSpace(c.Query("end_time")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.BadRequest(c, "invalid end_time format, use RFC3339")
			return filters, false
		}
		filters.EndTime = &t
	}
	return filters, true
}
