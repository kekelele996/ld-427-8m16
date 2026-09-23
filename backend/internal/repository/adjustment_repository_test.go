package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/model"
)

func TestAdjustmentRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewAdjustmentRepository(newTestDB(t))

	adjustment := &model.BudgetAdjustment{
		BudgetSheetID:  1,
		ProposedAmount: 1500,
		Reason:         "增加主材预算",
		Status:         constants.AdjustmentStatusPending,
		BudgetVersion:  1,
		SubmittedByID:  2,
	}
	if err := repo.Create(ctx, adjustment); err != nil {
		t.Fatalf("create: %v", err)
	}
	if adjustment.ID == 0 {
		t.Fatalf("expected id to be assigned")
	}

	got, err := repo.FindByID(ctx, adjustment.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got.ProposedAmount != 1500 || got.BudgetVersion != 1 {
		t.Fatalf("unexpected adjustment: %+v", got)
	}

	pending, err := repo.FindPendingByBudgetID(ctx, 1)
	if err != nil {
		t.Fatalf("find pending: %v", err)
	}
	if pending.ID != adjustment.ID {
		t.Fatalf("pending id = %d, want %d", pending.ID, adjustment.ID)
	}

	list, err := repo.ListByBudgetID(ctx, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}

	reviewerID := uint(3)
	adjustment.Status = constants.AdjustmentStatusApproved
	adjustment.ReviewedByID = &reviewerID
	adjustment.ReviewComment = "同意"
	if err := repo.Update(ctx, adjustment); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := repo.FindPendingByBudgetID(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestAdjustmentRepositoryFindByIDNotFound(t *testing.T) {
	repo := NewAdjustmentRepository(newTestDB(t))
	if _, err := repo.FindByID(context.Background(), 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestAdjustmentRepositorySinglePendingPerBudget(t *testing.T) {
	ctx := context.Background()
	repo := NewAdjustmentRepository(newTestDB(t))

	first := &model.BudgetAdjustment{BudgetSheetID: 1, ProposedAmount: 1500, Reason: "第一次", Status: constants.AdjustmentStatusPending, BudgetVersion: 1, SubmittedByID: 2}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	second := &model.BudgetAdjustment{BudgetSheetID: 1, ProposedAmount: 1800, Reason: "第二次", Status: constants.AdjustmentStatusPending, BudgetVersion: 1, SubmittedByID: 2}
	if err := repo.Create(ctx, second); err == nil {
		t.Fatalf("expected unique violation for second pending adjustment")
	}

	// 非待审状态不受限制。
	first.Status = constants.AdjustmentStatusRejected
	if err := repo.Update(ctx, first); err != nil {
		t.Fatalf("update first: %v", err)
	}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("create after reject: %v", err)
	}
}
