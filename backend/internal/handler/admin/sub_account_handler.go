package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/SSYC-LJS/sub2api/internal/handler/dto"
	"github.com/SSYC-LJS/sub2api/internal/pkg/pagination"
	"github.com/SSYC-LJS/sub2api/internal/pkg/response"
	"github.com/SSYC-LJS/sub2api/internal/server/middleware"
	"github.com/SSYC-LJS/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type SubAccountHandler struct {
	service *service.SubAccountService
}

func NewSubAccountHandler(s *service.SubAccountService) *SubAccountHandler {
	return &SubAccountHandler{service: s}
}

type subAccountUpsertRequest struct {
	ChildUserID          int64   `json:"child_user_id" binding:"required"`
	AllocatedQuota       float64 `json:"allocated_quota"`
	WeeklyAllocatedQuota float64 `json:"weekly_allocated_quota"`
}

type subAccountQuotaRequest struct {
	AllocatedQuota       float64 `json:"allocated_quota"`
	WeeklyAllocatedQuota float64 `json:"weekly_allocated_quota"`
}

type subAccountResponse struct {
	ID                   int64      `json:"id"`
	ParentUserID         int64      `json:"parent_user_id"`
	ChildUserID          int64      `json:"child_user_id"`
	AllocatedQuota       float64    `json:"allocated_quota"`
	UsedQuota            float64    `json:"used_quota"`
	RemainingQuota       float64    `json:"remaining_quota"`
	WeeklyAllocatedQuota float64    `json:"weekly_allocated_quota"`
	WeeklyUsedQuota      float64    `json:"weekly_used_quota"`
	WeeklyRemainingQuota float64    `json:"weekly_remaining_quota"`
	WeeklyWindowStart    *time.Time `json:"weekly_window_start,omitempty"`
	TotalRemainingQuota  float64    `json:"total_remaining_quota"`
	Status               string     `json:"status"`
	ChildUser            *dto.User  `json:"child,omitempty"`
}

func parseSubAccountPagination(c *gin.Context) pagination.PaginationParams {
	params := pagination.DefaultPagination()
	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && page > 0 {
		params.Page = page
	}
	if pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil && pageSize > 0 {
		params.PageSize = pageSize
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	params.SortBy = c.Query("sort_by")
	params.SortOrder = c.Query("sort_order")
	return params
}

func subAccountFromService(rel service.SubAccountRelation) subAccountResponse {
	resp := subAccountResponse{ID: rel.ID, ParentUserID: rel.ParentUserID, ChildUserID: rel.ChildUserID, AllocatedQuota: rel.AllocatedQuota, UsedQuota: rel.UsedQuota, RemainingQuota: rel.RemainingQuota(), WeeklyAllocatedQuota: rel.WeeklyAllocatedQuota, WeeklyUsedQuota: rel.WeeklyUsedQuota, WeeklyRemainingQuota: rel.WeeklyRemainingQuota(), WeeklyWindowStart: rel.WeeklyWindowStart, TotalRemainingQuota: rel.TotalRemainingQuota(), Status: rel.Status}
	if rel.ChildUser != nil {
		resp.ChildUser = dto.UserFromService(rel.ChildUser)
	}
	return resp
}

func (h *SubAccountHandler) currentUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		return 0, false
	}
	return subject.UserID, true
}

func (h *SubAccountHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrParentAccountRequired):
		response.Error(c, http.StatusForbidden, "仅母账号可访问子账号管理")
	case errors.Is(err, service.ErrSubAccountNotFound):
		response.Error(c, http.StatusNotFound, "子账号关系不存在")
	case errors.Is(err, service.ErrSubAccountSelfLink):
		response.Error(c, http.StatusBadRequest, "不能将自己添加为子账号")
	case errors.Is(err, service.ErrSubAccountAlreadyLinked):
		response.Error(c, http.StatusConflict, "该账号已绑定母账号")
	default:
		response.InternalError(c, err.Error())
	}
}

func (h *SubAccountHandler) List(c *gin.Context) {
	parentID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	params := parseSubAccountPagination(c)
	items, page, err := h.service.List(c.Request.Context(), parentID, params)
	if err != nil {
		h.handleError(c, err)
		return
	}
	out := make([]subAccountResponse, 0, len(items))
	for _, item := range items {
		out = append(out, subAccountFromService(item))
	}
	response.Success(c, gin.H{"items": out, "pagination": page})
}

func (h *SubAccountHandler) Add(c *gin.Context) {
	parentID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	var req subAccountUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数无效")
		return
	}
	rel, err := h.service.Add(c.Request.Context(), parentID, service.SubAccountUpsertInput{ChildUserID: req.ChildUserID, AllocatedQuota: req.AllocatedQuota, WeeklyAllocatedQuota: req.WeeklyAllocatedQuota})
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, subAccountFromService(*rel))
}

func (h *SubAccountHandler) SearchCandidates(c *gin.Context) {
	parentID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	items, page, err := h.service.SearchCandidates(c.Request.Context(), parentID, c.Query("q"), parseSubAccountPagination(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	out := make([]*dto.User, 0, len(items))
	for i := range items {
		out = append(out, dto.UserFromService(&items[i]))
	}
	response.Success(c, gin.H{"items": out, "pagination": page})
}

func (h *SubAccountHandler) UpdateQuota(c *gin.Context) {
	parentID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	childID, err := strconv.ParseInt(c.Param("child_id"), 10, 64)
	if err != nil || childID <= 0 {
		response.BadRequest(c, "子账号ID无效")
		return
	}
	var req subAccountQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数无效")
		return
	}
	rel, err := h.service.UpdateQuota(c.Request.Context(), parentID, childID, req.AllocatedQuota, req.WeeklyAllocatedQuota)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, subAccountFromService(*rel))
}

func (h *SubAccountHandler) Remove(c *gin.Context) {
	parentID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	childID, err := strconv.ParseInt(c.Param("child_id"), 10, 64)
	if err != nil || childID <= 0 {
		response.BadRequest(c, "子账号ID无效")
		return
	}
	if err := h.service.Remove(c.Request.Context(), parentID, childID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

func parseSubAccountUsageTimeRange(c *gin.Context) (service.UsageSummaryFilters, bool) {
	layout := "2006-01-02"
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -7)
	if raw := c.Query("start_date"); raw != "" {
		parsed, err := time.ParseInLocation(layout, raw, time.Local)
		if err != nil {
			response.BadRequest(c, "开始日期无效")
			return service.UsageSummaryFilters{}, false
		}
		start = parsed
	}
	if raw := c.Query("end_date"); raw != "" {
		parsed, err := time.ParseInLocation(layout, raw, time.Local)
		if err != nil {
			response.BadRequest(c, "结束日期无效")
			return service.UsageSummaryFilters{}, false
		}
		end = parsed.Add(24 * time.Hour)
	}
	if !end.After(start) {
		response.BadRequest(c, "结束日期必须晚于开始日期")
		return service.UsageSummaryFilters{}, false
	}
	return service.UsageSummaryFilters{StartTime: start, EndTime: end}, true
}

func (h *SubAccountHandler) UsageSummary(c *gin.Context) {
	parentID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	var childID int64
	if raw := c.Query("child_user_id"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.BadRequest(c, "子账号ID无效")
			return
		}
		childID = v
	}
	filters, ok := parseSubAccountUsageTimeRange(c)
	if !ok {
		return
	}
	summary, err := h.service.UsageSummary(c.Request.Context(), parentID, childID, filters)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, summary)
}

func (h *SubAccountHandler) Usage(c *gin.Context) {
	parentID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	var childID int64
	if raw := c.Query("child_user_id"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.BadRequest(c, "子账号ID无效")
			return
		}
		childID = v
	}
	items, page, err := h.service.ListUsage(c.Request.Context(), parentID, childID, parseSubAccountPagination(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	out := make([]*dto.UsageLog, 0, len(items))
	for i := range items {
		out = append(out, dto.UsageLogFromService(&items[i]))
	}
	response.Success(c, gin.H{"items": out, "pagination": page})
}
