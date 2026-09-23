package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/dto"
	"github.com/renovation/renovation-budget-api/internal/middleware"
	"github.com/renovation/renovation-budget-api/internal/response"
	"github.com/renovation/renovation-budget-api/internal/service"
)

// BudgetHandler 预算表处理层。
type BudgetHandler struct {
	service       *service.BudgetService
	adjustService *service.BudgetAdjustmentService
	logger        *slog.Logger
}

// NewBudgetHandler 构造预算表处理层。
func NewBudgetHandler(budgetService *service.BudgetService, adjustService *service.BudgetAdjustmentService, logger *slog.Logger) *BudgetHandler {
	return &BudgetHandler{service: budgetService, adjustService: adjustService, logger: logger}
}

// Create 创建预算表。
// @Summary 创建预算表
// @Tags budgets
// @Accept json
// @Produce json
// @Param request body dto.CreateBudgetRequest true "创建请求"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 403 {object} response.Body
// @Security BearerAuth
// @Router /budgets [post]
func (h *BudgetHandler) Create(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	var req dto.CreateBudgetRequest
	if !bindJSON(c, &req) {
		return
	}
	sheet, err := h.service.Create(context.Background(), actor, req)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, sheet)
}

// List 查询预算表列表。
// @Summary 查询预算表列表
// @Tags budgets
// @Produce json
// @Param project_id query string false "项目ID"
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Security BearerAuth
// @Router /budgets [get]
func (h *BudgetHandler) List(c *gin.Context) {
	var filter dto.BudgetFilter
	if !bindQuery(c, &filter) {
		return
	}
	sheets, total, err := h.service.List(context.Background(), filter)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, dto.PageResult{List: sheets, Total: total, Page: filter.Page, PageSize: filter.PageSize})
}

// Get 获取预算表详情。
// @Summary 获取预算表详情
// @Tags budgets
// @Produce json
// @Param id path int true "预算表ID"
// @Success 200 {object} response.Body
// @Failure 404 {object} response.Body
// @Security BearerAuth
// @Router /budgets/{id} [get]
func (h *BudgetHandler) Get(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	sheet, err := h.service.Get(context.Background(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, sheet)
}

// Update 更新预算表。
// 名称/状态直接生效；一旦请求携带 total_amount，则不允许直接改总额，
// 而是转入预算调整单审批流程（reason 必填），返回待审调整单。
// @Summary 更新预算表
// @Tags budgets
// @Accept json
// @Produce json
// @Param id path int true "预算表ID"
// @Param request body dto.UpdateBudgetRequest true "更新请求"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Security BearerAuth
// @Router /budgets/{id} [put]
func (h *BudgetHandler) Update(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateBudgetRequest
	if !bindJSON(c, &req) {
		return
	}
	// 总额变更统一纳入调整单审批这道门。
	if req.TotalAmount > 0 {
		if req.Reason == "" {
			response.Abort(c, http.StatusBadRequest, constants.CodeBadRequest, "reason is required when total_amount is provided")
			return
		}
		adjustment, err := h.adjustService.Submit(context.Background(), actor, id, dto.CreateBudgetAdjustmentRequest{
			TotalAmount: req.TotalAmount,
			Reason:      req.Reason,
		})
		if err != nil {
			handleError(c, err)
			return
		}
		response.OK(c, adjustment)
		return
	}
	sheet, err := h.service.Update(context.Background(), actor, id, req)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, sheet)
}

// Adjust 旧版预算总额调整入口，现统一转入调整单审批流程：提交一张待审调整单。
// @Summary 提交预算调整单（旧入口）
// @Tags budgets
// @Accept json
// @Produce json
// @Param id path int true "预算表ID"
// @Param request body dto.CreateBudgetAdjustmentRequest true "调整请求"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 409 {object} response.Body
// @Security BearerAuth
// @Router /budgets/{id}/adjust [post]
func (h *BudgetHandler) Adjust(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req dto.CreateBudgetAdjustmentRequest
	if !bindJSON(c, &req) {
		return
	}
	adjustment, err := h.adjustService.Submit(context.Background(), actor, id, req)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, adjustment)
}

// Delete 删除预算表。
// @Summary 删除预算表
// @Tags budgets
// @Produce json
// @Param id path int true "预算表ID"
// @Success 200 {object} response.Body
// @Failure 409 {object} response.Body
// @Security BearerAuth
// @Router /budgets/{id} [delete]
func (h *BudgetHandler) Delete(c *gin.Context) {
	actor, _ := middleware.CurrentActor(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(context.Background(), actor, id); err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, nil)
}
