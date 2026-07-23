package handler

import (
	"strconv"

	"github.com/SSYC-LJS/sub2api/internal/pkg/pagination"
	"github.com/SSYC-LJS/sub2api/internal/pkg/response"
	middleware2 "github.com/SSYC-LJS/sub2api/internal/server/middleware"
	"github.com/SSYC-LJS/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ImageCanvasHandler struct {
	service *service.ImageCanvasService
}

func NewImageCanvasHandler(service *service.ImageCanvasService) *ImageCanvasHandler {
	return &ImageCanvasHandler{service: service}
}

func (h *ImageCanvasHandler) ListHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.ListHistory(c.Request.Context(), subject.UserID, pagination.PaginationParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *ImageCanvasHandler) CreateHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req service.CreateImageCanvasHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.service.CreateHistory(c.Request.Context(), subject.UserID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *ImageCanvasHandler) DeleteHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid history id")
		return
	}
	if err := h.service.DeleteHistory(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
