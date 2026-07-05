package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type SubAccountHandler struct {
	service *service.SubAccountService
}

func NewSubAccountHandler(s *service.SubAccountService) *SubAccountHandler {
	return &SubAccountHandler{service: s}
}

type subAccountUpsertRequest struct {
	ChildUserID    int64   `json:"child_user_id" binding:"required"`
	AllocatedQuota float64 `json:"allocated_quota"`
}

type subAccountQuotaRequest struct {
	AllocatedQuota float64 `json:"allocated_quota"`
}

type subAccountResponse struct {
	ID             int64     `json:"id"`
	ParentUserID   int64     `json:"parent_user_id"`
	ChildUserID    int64     `json:"child_user_id"`
	AllocatedQuota float64   `json:"allocated_quota"`
	UsedQuota      float64   `json:"used_quota"`
	RemainingQuota float64   `json:"remaining_quota"`
	Status         string    `json:"status"`
	ChildUser      *dto.User `json:"child,omitempty"`
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
	resp := subAccountResponse{ID: rel.ID, ParentUserID: rel.ParentUserID, ChildUserID: rel.ChildUserID, AllocatedQuota: rel.AllocatedQuota, UsedQuota: rel.UsedQuota, RemainingQuota: rel.RemainingQuota(), Status: rel.Status}
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
	rel, err := h.service.Add(c.Request.Context(), parentID, service.SubAccountUpsertInput{ChildUserID: req.ChildUserID, AllocatedQuota: req.AllocatedQuota})
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
	rel, err := h.service.UpdateQuota(c.Request.Context(), parentID, childID, req.AllocatedQuota)
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
