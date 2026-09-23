package dto

import "github.com/renovation/renovation-budget-api/internal/constants"

// CreateBudgetRequest 创建预算表请求。
type CreateBudgetRequest struct {
	ProjectID   string                 `json:"project_id" binding:"required"`
	Name        string                 `json:"name" binding:"required,max=128"`
	TotalAmount float64                `json:"total_amount" binding:"required,gt=0"`
	Status      constants.BudgetStatus `json:"status" binding:"omitempty,budget_status"`
}

// UpdateBudgetRequest 更新预算表请求。
// 名称、状态直接更新；一旦携带大于 0 的 total_amount，则不再直接改总额，
// 而是作为预算调整单走审批流程（此时 reason 必填）。
type UpdateBudgetRequest struct {
	Name        string                 `json:"name" binding:"omitempty,max=128"`
	TotalAmount float64                `json:"total_amount" binding:"omitempty,gte=0"`
	Reason      string                 `json:"reason" binding:"omitempty,max=512"`
	Status      constants.BudgetStatus `json:"status" binding:"omitempty,budget_status"`
}
