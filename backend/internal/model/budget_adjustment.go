package model

import (
	"time"

	"github.com/renovation/renovation-budget-api/internal/constants"
)

// BudgetAdjustment 预算调整单。所有预算总额变更必须先提交调整单，
// 经财务经理审批通过后金额才生效；审批后单据保留审批人、意见和拟调金额。
type BudgetAdjustment struct {
	ID              uint                             `gorm:"primaryKey" json:"id"`
	BudgetSheetID   uint                             `gorm:"not null;index" json:"budget_sheet_id"`
	ProposedAmount  float64                          `gorm:"not null" json:"proposed_amount"`
	Reason          string                           `gorm:"size:512;not null" json:"reason"`
	BaseVersion     int                              `gorm:"not null" json:"base_version"`
	Status          constants.BudgetAdjustmentStatus `gorm:"size:32;not null;default:Pending;index" json:"status"`
	ApplicantID     uint                             `gorm:"not null" json:"applicant_id"`
	ApprovedByID    *uint                            `json:"approved_by_id"`
	ApprovalComment string                           `gorm:"size:512" json:"approval_comment"`
	ApprovedAt      *time.Time                       `json:"approved_at"`
	CreatedAt       time.Time                        `json:"created_at"`
	UpdatedAt       time.Time                        `json:"updated_at"`
}
