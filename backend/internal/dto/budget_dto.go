package dto

import (
	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/model"
)

// CreateBudgetRequest 创建预算表请求。
type CreateBudgetRequest struct {
	ProjectID   string                 `json:"project_id" binding:"required"`
	Name        string                 `json:"name" binding:"required,max=128"`
	TotalAmount float64                `json:"total_amount" binding:"required,gt=0"`
	Status      constants.BudgetStatus `json:"status" binding:"omitempty,budget_status"`
}

// UpdateBudgetRequest 更新预算表请求。总额变更不直接生效，会转为待审批的预算调整单。
type UpdateBudgetRequest struct {
	Name         string                 `json:"name" binding:"omitempty,max=128"`
	TotalAmount  float64                `json:"total_amount" binding:"omitempty,gte=0"`
	Status       constants.BudgetStatus `json:"status" binding:"omitempty,budget_status"`
	AdjustReason string                 `json:"adjust_reason" binding:"omitempty,max=512"`
}

// UpdateBudgetResponse 更新预算表响应；总额变更已转为待审批调整单时一并返回。
type UpdateBudgetResponse struct {
	Sheet      *model.BudgetSheet      `json:"sheet"`
	Adjustment *model.BudgetAdjustment `json:"adjustment,omitempty"`
}

// AdjustBudgetRequest 预算调整请求。通过该入口提交的调整同样进入审批流程。
type AdjustBudgetRequest struct {
	TotalAmount float64 `json:"total_amount" binding:"required,gt=0"`
	Reason      string  `json:"reason" binding:"required,max=512"`
}
