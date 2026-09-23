package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PlanCatalogHandler struct{ service *service.PlanCatalogService }

func NewPlanCatalogHandler(svc *service.PlanCatalogService) *PlanCatalogHandler {
	return &PlanCatalogHandler{service: svc}
}

func (h *PlanCatalogHandler) List(c *gin.Context) {
	items, err := h.service.ListPublished(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

type AdminPlanCatalogHandler struct{ service *service.PlanCatalogService }

func NewAdminPlanCatalogHandler(svc *service.PlanCatalogService) *AdminPlanCatalogHandler {
	return &AdminPlanCatalogHandler{service: svc}
}

func (h *AdminPlanCatalogHandler) List(c *gin.Context) {
	items, err := h.service.ListAdmin(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *AdminPlanCatalogHandler) Create(c *gin.Context) {
	var input service.PlanCatalogInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid plan catalog payload")
		return
	}
	item, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *AdminPlanCatalogHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid plan catalog item id")
		return
	}
	var input service.PlanCatalogInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid plan catalog payload")
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AdminPlanCatalogHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid plan catalog item id")
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
