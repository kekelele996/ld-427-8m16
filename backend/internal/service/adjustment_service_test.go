package service

import (
	"context"
	"errors"
	"testing"

	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/dto"
	"github.com/renovation/renovation-budget-api/internal/model"
)

func newAdjustmentTestService() (*AdjustmentService, *fakeBudgetRepo, *fakeAdjustmentRepo) {
	budgetRepo := newFakeBudgetRepo()
	adjustmentRepo := newFakeAdjustmentRepo()
	audit := NewAuditService(newFakeAuditRepo(), testLogger())
	return NewAdjustmentService(adjustmentRepo, budgetRepo, audit, nil, testLogger()), budgetRepo, adjustmentRepo
}

func seedSheet(t *testing.T, ctx context.Context, repo *fakeBudgetRepo, total float64) *model.BudgetSheet {
	t.Helper()
	sheet := &model.BudgetSheet{
		ProjectID:       "p-1",
		Name:            "预算",
		TotalAmount:     total,
		AvailableAmount: total,
		Status:          constants.BudgetStatusActive,
		CreatedByID:     1,
		Version:         1,
	}
	if err := repo.Create(ctx, sheet); err != nil {
		t.Fatalf("seed sheet: %v", err)
	}
	return sheet
}

func TestAdjustmentSubmitAndApprove(t *testing.T) {
	ctx := context.Background()
	svc, budgetRepo, _ := newAdjustmentTestService()
	sheet := seedSheet(t, ctx, budgetRepo, 1000)

	pm := model.Actor{UserID: 2, Username: "project"}
	adjustment, err := svc.Submit(ctx, pm, sheet.ID, dto.SubmitAdjustmentRequest{ProposedAmount: 1500, Reason: "增加主材预算"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if adjustment.Status != constants.AdjustmentStatusPending || adjustment.BudgetVersion != 1 || adjustment.SubmittedByID != 2 {
		t.Fatalf("unexpected adjustment: %+v", adjustment)
	}

	fm := model.Actor{UserID: 3, Username: "finance"}
	approved, err := svc.Approve(ctx, fm, adjustment.ID, dto.ApproveAdjustmentRequest{ReviewComment: "同意"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != constants.AdjustmentStatusApproved {
		t.Fatalf("status = %q, want Approved", approved.Status)
	}
	if approved.ReviewedByID == nil || *approved.ReviewedByID != 3 || approved.ReviewComment != "同意" {
		t.Fatalf("review info not kept: %+v", approved)
	}

	got, err := budgetRepo.FindByID(ctx, sheet.ID)
	if err != nil {
		t.Fatalf("find sheet: %v", err)
	}
	if got.TotalAmount != 1500 || got.Version != 2 || got.AvailableAmount != 1500 {
		t.Fatalf("unexpected sheet: total=%v version=%d available=%v", got.TotalAmount, got.Version, got.AvailableAmount)
	}
}

func TestAdjustmentApproveStaleVersion(t *testing.T) {
	ctx := context.Background()
	svc, budgetRepo, _ := newAdjustmentTestService()
	sheet := seedSheet(t, ctx, budgetRepo, 1000)

	adjustment, err := svc.Submit(ctx, model.Actor{UserID: 2}, sheet.ID, dto.SubmitAdjustmentRequest{ProposedAmount: 1500, Reason: "第一次"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	// 提交后预算被其他流程改过，版本前进。
	sheet.TotalAmount = 1200
	sheet.Version = 2
	if err := budgetRepo.Update(ctx, sheet); err != nil {
		t.Fatalf("bump version: %v", err)
	}

	if _, err := svc.Approve(ctx, model.Actor{UserID: 3}, adjustment.ID, dto.ApproveAdjustmentRequest{}); !errors.Is(err, ErrStaleVersion) {
		t.Fatalf("error = %v, want ErrStaleVersion", err)
	}
}

func TestAdjustmentSinglePending(t *testing.T) {
	ctx := context.Background()
	svc, budgetRepo, _ := newAdjustmentTestService()
	sheet := seedSheet(t, ctx, budgetRepo, 1000)

	if _, err := svc.Submit(ctx, model.Actor{UserID: 2}, sheet.ID, dto.SubmitAdjustmentRequest{ProposedAmount: 1500, Reason: "第一次"}); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if _, err := svc.Submit(ctx, model.Actor{UserID: 2}, sheet.ID, dto.SubmitAdjustmentRequest{ProposedAmount: 1800, Reason: "第二次"}); !errors.Is(err, ErrPendingAdjustmentExists) {
		t.Fatalf("error = %v, want ErrPendingAdjustmentExists", err)
	}
}

func TestAdjustmentRejectKeepsReviewInfo(t *testing.T) {
	ctx := context.Background()
	svc, budgetRepo, _ := newAdjustmentTestService()
	sheet := seedSheet(t, ctx, budgetRepo, 1000)

	adjustment, err := svc.Submit(ctx, model.Actor{UserID: 2}, sheet.ID, dto.SubmitAdjustmentRequest{ProposedAmount: 1500, Reason: "增加主材预算"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	rejected, err := svc.Reject(ctx, model.Actor{UserID: 3}, adjustment.ID, dto.RejectAdjustmentRequest{ReviewComment: "超出季度额度"})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != constants.AdjustmentStatusRejected {
		t.Fatalf("status = %q, want Rejected", rejected.Status)
	}
	if rejected.ReviewedByID == nil || *rejected.ReviewedByID != 3 || rejected.ReviewComment != "超出季度额度" || rejected.ProposedAmount != 1500 {
		t.Fatalf("review info not kept: %+v", rejected)
	}

	// 驳回后预算不变，且可以再次提交。
	got, err := budgetRepo.FindByID(ctx, sheet.ID)
	if err != nil {
		t.Fatalf("find sheet: %v", err)
	}
	if got.TotalAmount != 1000 || got.Version != 1 {
		t.Fatalf("sheet changed after reject: total=%v version=%d", got.TotalAmount, got.Version)
	}
	if _, err := svc.Submit(ctx, model.Actor{UserID: 2}, sheet.ID, dto.SubmitAdjustmentRequest{ProposedAmount: 1600, Reason: "重新提交"}); err != nil {
		t.Fatalf("resubmit after reject: %v", err)
	}
}

func TestAdjustmentApproveNonPending(t *testing.T) {
	ctx := context.Background()
	svc, budgetRepo, _ := newAdjustmentTestService()
	sheet := seedSheet(t, ctx, budgetRepo, 1000)

	adjustment, err := svc.Submit(ctx, model.Actor{UserID: 2}, sheet.ID, dto.SubmitAdjustmentRequest{ProposedAmount: 1500, Reason: "调整"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Approve(ctx, model.Actor{UserID: 3}, adjustment.ID, dto.ApproveAdjustmentRequest{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := svc.Approve(ctx, model.Actor{UserID: 3}, adjustment.ID, dto.ApproveAdjustmentRequest{}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("error = %v, want ErrInvalidState", err)
	}
}
