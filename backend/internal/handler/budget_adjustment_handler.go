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

// BudgetAdjustmentHandler 预算调整单处理层。
type BudgetAdjustmentHandler struct {
	service *service.BudgetAdjustmentService
	logger  *slog.Logger
}

// NewBudgetAdjustmentHandler 构造预算调整单处理层。
func NewBudgetAdjustmentHandler(service *service.BudgetAdjustmentService, logger *slog.Logger) *BudgetAdjustmentHandler {
	return &BudgetAdjustmentHandler{service: service, logger: logger}
}

// Submit 提交预算调整单。
// @Summary 提交预算调整单
// @Tags budget-adjustments
// @Accept json
// @Produce json
// @Param id path int true "预算表ID"
// @Param request body dto.CreateBudgetAdjustmentRequest true "调整单请求"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 409 {object} response.Body
// @Security BearerAuth
// @Router /budgets/{id}/adjustments [post]
func (h *BudgetAdjustmentHandler) Submit(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	budgetID, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req dto.CreateBudgetAdjustmentRequest
	if !bindJSON(c, &req) {
		return
	}
	adjustment, err := h.service.Submit(context.Background(), actor, budgetID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, adjustment)
}

// ListByBudget 查询指定预算表下的调整单列表。
// @Summary 查询指定预算表的调整单列表
// @Tags budget-adjustments
// @Produce json
// @Param id path int true "预算表ID"
// @Param status query string false "审批状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Security BearerAuth
// @Router /budgets/{id}/adjustments [get]
func (h *BudgetAdjustmentHandler) ListByBudget(c *gin.Context) {
	budgetID, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var filter dto.BudgetAdjustmentFilter
	if !bindQuery(c, &filter) {
		return
	}
	filter.BudgetSheetID = budgetID
	adjustments, total, err := h.service.List(context.Background(), filter)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, dto.PageResult{List: adjustments, Total: total, Page: filter.Page, PageSize: filter.PageSize})
}

// Get 获取预算调整单详情。
// @Summary 获取预算调整单详情
// @Tags budget-adjustments
// @Produce json
// @Param id path int true "调整单ID"
// @Success 200 {object} response.Body
// @Failure 404 {object} response.Body
// @Security BearerAuth
// @Router /budget-adjustments/{id} [get]
func (h *BudgetAdjustmentHandler) Get(c *gin.Context) {
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

// List 查询预算调整单列表。
// @Summary 查询预算调整单列表
// @Tags budget-adjustments
// @Produce json
// @Param budget_sheet_id query int false "预算表ID"
// @Param status query string false "审批状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Security BearerAuth
// @Router /budget-adjustments [get]
func (h *BudgetAdjustmentHandler) List(c *gin.Context) {
	var filter dto.BudgetAdjustmentFilter
	if !bindQuery(c, &filter) {
		return
	}
	adjustments, total, err := h.service.List(context.Background(), filter)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, dto.PageResult{List: adjustments, Total: total, Page: filter.Page, PageSize: filter.PageSize})
}

// Approve 财务经理审批通过预算调整单。
// @Summary 审批通过预算调整单
// @Tags budget-adjustments
// @Accept json
// @Produce json
// @Param id path int true "调整单ID"
// @Param request body dto.ApproveBudgetAdjustmentRequest true "审批请求"
// @Success 200 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 409 {object} response.Body
// @Security BearerAuth
// @Router /budget-adjustments/{id}/approve [post]
func (h *BudgetAdjustmentHandler) Approve(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req dto.ApproveBudgetAdjustmentRequest
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

// Reject 财务经理驳回预算调整单。
// @Summary 驳回预算调整单
// @Tags budget-adjustments
// @Accept json
// @Produce json
// @Param id path int true "调整单ID"
// @Param request body dto.RejectBudgetAdjustmentRequest true "驳回请求"
// @Success 200 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 409 {object} response.Body
// @Security BearerAuth
// @Router /budget-adjustments/{id}/reject [post]
func (h *BudgetAdjustmentHandler) Reject(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req dto.RejectBudgetAdjustmentRequest
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
