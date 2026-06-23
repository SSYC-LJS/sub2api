package admin

import (
	"errors"
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
