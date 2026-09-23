package model

import (
	"time"

	"github.com/renovation/renovation-budget-api/internal/constants"
)

// BudgetAdjustment 预算调整单。预算总额变更必须先提交调整，须经财务经理审批后生效。
// budget_sheet_id 上的部分唯一索引保证同一份预算同一时间只有一张待审单。
type BudgetAdjustment struct {
	ID             uint                       `gorm:"primaryKey" json:"id"`
	BudgetSheetID  uint                       `gorm:"not null;index;uniqueIndex:idx_budget_adjustments_pending,where:status = 'Pending'" json:"budget_sheet_id"`
	ProposedAmount float64                    `gorm:"not null" json:"proposed_amount"`
	Reason         string                     `gorm:"size:512;not null" json:"reason"`
	Status         constants.AdjustmentStatus `gorm:"size:32;not null;default:Pending" json:"status"`
	BudgetVersion  int                        `gorm:"not null" json:"budget_version"`
	SubmittedByID  uint                       `gorm:"not null" json:"submitted_by_id"`
	ReviewedByID   *uint                      `json:"reviewed_by_id"`
	ReviewComment  string                     `gorm:"size:512" json:"review_comment"`
	ReviewedAt     *time.Time                 `json:"reviewed_at"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
}
