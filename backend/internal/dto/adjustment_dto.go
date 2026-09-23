package dto

// SubmitAdjustmentRequest 提交预算调整单请求。
type SubmitAdjustmentRequest struct {
	ProposedAmount float64 `json:"proposed_amount" binding:"required,gt=0"`
	Reason         string  `json:"reason" binding:"required,max=512"`
}

// ApproveAdjustmentRequest 批准预算调整单请求。
type ApproveAdjustmentRequest struct {
	ReviewComment string `json:"review_comment" binding:"omitempty,max=512"`
}

// RejectAdjustmentRequest 驳回预算调整单请求。
type RejectAdjustmentRequest struct {
	ReviewComment string `json:"review_comment" binding:"required,max=512"`
}
