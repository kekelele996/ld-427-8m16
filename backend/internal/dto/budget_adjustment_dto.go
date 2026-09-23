package dto

import "github.com/renovation/renovation-budget-api/internal/constants"

// CreateBudgetAdjustmentRequest 提交预算调整单请求。
// 项目经理基于当前预算版本填写拟调金额（调整后的预算总额）和调整原因。
type CreateBudgetAdjustmentRequest struct {
	TotalAmount float64 `json:"total_amount" binding:"required,gt=0"`
	Reason      string  `json:"reason" binding:"required,max=512"`
}

// ApproveBudgetAdjustmentRequest 审批通过预算调整单请求。
type ApproveBudgetAdjustmentRequest struct {
	ApprovalComment string `json:"approval_comment" binding:"omitempty,max=512"`
}

// RejectBudgetAdjustmentRequest 驳回预算调整单请求。
type RejectBudgetAdjustmentRequest struct {
	ApprovalComment string `json:"approval_comment" binding:"required,max=512"`
}

// BudgetAdjustmentFilter 预算调整单列表查询参数。
type BudgetAdjustmentFilter struct {
	BudgetSheetID uint                             `form:"budget_sheet_id"`
	Status        constants.BudgetAdjustmentStatus `form:"status" binding:"omitempty,budget_adjustment_status"`
	Page          int                              `form:"page" binding:"omitempty,min=1"`
	PageSize      int                              `form:"page_size" binding:"omitempty,min=1,max=100"`
}
