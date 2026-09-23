package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/model"
)

func TestBudgetAdjustmentRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	sheetRepo := NewBudgetRepository(db)
	repo := NewBudgetAdjustmentRepository(db)

	sheet := &model.BudgetSheet{ProjectID: "p-200", Name: "预算", TotalAmount: 1000, AvailableAmount: 1000, Status: constants.BudgetStatusActive, CreatedByID: 1, Version: 1}
	if err := sheetRepo.Create(ctx, sheet); err != nil {
		t.Fatalf("create sheet: %v", err)
	}

	first := &model.BudgetAdjustment{
		BudgetSheetID:  sheet.ID,
		ProposedAmount: 1500,
		Reason:         "加项",
		BaseVersion:    1,
		Status:         constants.BudgetAdjustmentStatusPending,
		ApplicantID:    2,
	}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create adjustment: %v", err)
	}

	pending, err := repo.FindPendingByBudgetID(ctx, sheet.ID)
	if err != nil {
		t.Fatalf("find pending: %v", err)
	}
	if pending.ID != first.ID || pending.ProposedAmount != 1500 {
		t.Fatalf("unexpected pending adjustment: %+v", pending)
	}

	// 审批通过：金额生效保留审批人、意见与拟调金额。
	approver := uint(3)
	pending.Status = constants.BudgetAdjustmentStatusApproved
	pending.ApprovedByID = &approver
	pending.ApprovalComment = "同意"
	if err := repo.Update(ctx, pending); err != nil {
		t.Fatalf("approve update: %v", err)
	}
	if _, err := repo.FindPendingByBudgetID(ctx, sheet.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending after approve error = %v, want ErrNotFound", err)
	}

	// 已处理的单据仍可查询并保留审批信息。
	processed, err := repo.FindByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("find processed: %v", err)
	}
	if processed.Status != constants.BudgetAdjustmentStatusApproved || processed.ApprovedByID == nil || *processed.ApprovedByID != approver {
		t.Fatalf("processed adjustment lost approval info: %+v", processed)
	}
	if processed.ProposedAmount != 1500 || processed.ApprovalComment != "同意" {
		t.Fatalf("processed adjustment lost amount/comment: %+v", processed)
	}

	// 新一张待审单可再次提交。
	second := &model.BudgetAdjustment{
		BudgetSheetID:  sheet.ID,
		ProposedAmount: 1800,
		Reason:         "再加",
		BaseVersion:    2,
		Status:         constants.BudgetAdjustmentStatusPending,
		ApplicantID:    2,
	}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("create second adjustment: %v", err)
	}
	if _, err := repo.FindPendingByBudgetID(ctx, sheet.ID); err != nil {
		t.Fatalf("find second pending: %v", err)
	}

	list, total, err := repo.List(ctx, BudgetAdjustmentListFilter{BudgetSheetID: sheet.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("list total=%d len=%d", total, len(list))
	}
}
