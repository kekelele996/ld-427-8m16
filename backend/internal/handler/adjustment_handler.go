package handler

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/renovation/renovation-budget-api/internal/dto"
	"github.com/renovation/renovation-budget-api/internal/middleware"
	"github.com/renovation/renovation-budget-api/internal/response"
	"github.com/renovation/renovation-budget-api/internal/service"
)

// AdjustmentHandler 预算调整单处理层。
type AdjustmentHandler struct {
	service *service.AdjustmentService
	logger  *slog.Logger
}

// NewAdjustmentHandler 构造预算调整单处理层。
func NewAdjustmentHandler(service *service.AdjustmentService, logger *slog.Logger) *AdjustmentHandler {
	return &AdjustmentHandler{service: service, logger: logger}
}

// Submit 提交预算调整单。
// @Summary 提交预算调整单
// @Tags budget-adjustments
// @Accept json
// @Produce json
// @Param id path int true "预算表ID"
// @Param request body dto.SubmitAdjustmentRequest true "调整请求"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Security BearerAuth
// @Router /budgets/{id}/adjustments [post]
func (h *AdjustmentHandler) Submit(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	budgetSheetID, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req dto.SubmitAdjustmentRequest
	if !bindJSON(c, &req) {
		return
	}
	adjustment, err := h.service.Submit(context.Background(), actor, budgetSheetID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, adjustment)
}

// List 查询预算表的调整单列表。
// @Summary 查询预算表的调整单列表
// @Tags budget-adjustments
// @Produce json
// @Param id path int true "预算表ID"
// @Success 200 {object} response.Body
// @Failure 404 {object} response.Body
// @Security BearerAuth
// @Router /budgets/{id}/adjustments [get]
func (h *AdjustmentHandler) List(c *gin.Context) {
	budgetSheetID, ok := pathUint(c, "id")
	if !ok {
		return
	}
	adjustments, err := h.service.List(context.Background(), budgetSheetID)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, adjustments)
}

// Get 获取预算调整单详情。
// @Summary 获取预算调整单详情
// @Tags budget-adjustments
// @Produce json
// @Param id path int true "调整单ID"
// @Success 200 {object} response.Body
// @Failure 404 {object} response.Body
// @Security BearerAuth
// @Router /adjustments/{id} [get]
func (h *AdjustmentHandler) Get(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	adjustment, err := h.service.Get(context.Background(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, adjustment)
}

// Approve 批准预算调整单。
// @Summary 批准预算调整单
// @Tags budget-adjustments
// @Accept json
// @Produce json
// @Param id path int true "调整单ID"
// @Param request body dto.ApproveAdjustmentRequest true "审批意见"
// @Success 200 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 409 {object} response.Body
// @Security BearerAuth
// @Router /adjustments/{id}/approve [post]
func (h *AdjustmentHandler) Approve(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req dto.ApproveAdjustmentRequest
	if !bindJSON(c, &req) {
		return
	}
	adjustment, err := h.service.Approve(context.Background(), actor, id, req)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, adjustment)
}

// Reject 驳回预算调整单。
// @Summary 驳回预算调整单
// @Tags budget-adjustments
// @Accept json
// @Produce json
// @Param id path int true "调整单ID"
// @Param request body dto.RejectAdjustmentRequest true "驳回意见"
// @Success 200 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 409 {object} response.Body
// @Security BearerAuth
// @Router /adjustments/{id}/reject [post]
func (h *AdjustmentHandler) Reject(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req dto.RejectAdjustmentRequest
	if !bindJSON(c, &req) {
		return
	}
	adjustment, err := h.service.Reject(context.Background(), actor, id, req)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, adjustment)
}
