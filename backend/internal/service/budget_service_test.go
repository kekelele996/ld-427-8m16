package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/dto"
	"github.com/renovation/renovation-budget-api/internal/model"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBudgetServiceCreateAndGet(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditService(newFakeAuditRepo(), testLogger())
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeItemRepo(), newFakeAdjustmentRepo(), audit, nil, testLogger())

	sheet, err := svc.Create(ctx, model.Actor{UserID: 1, Username: "admin"}, dto.CreateBudgetRequest{
		ProjectID:   "p-1",
		Name:        "整屋装修",
		TotalAmount: 20000,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sheet.Status != constants.BudgetStatusDraft {
		t.Fatalf("status = %q, want Draft", sheet.Status)
	}
	if sheet.AvailableAmount != 20000 {
		t.Fatalf("available = %v, want 20000", sheet.AvailableAmount)
	}

	got, err := svc.Get(ctx, sheet.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "整屋装修" {
		t.Fatalf("name = %q", got.Name)
	}
}

func TestBudgetServiceDeleteNonDraft(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditService(newFakeAuditRepo(), testLogger())
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeItemRepo(), newFakeAdjustmentRepo(), audit, nil, testLogger())

	sheet, err := svc.Create(ctx, model.Actor{UserID: 1}, dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000, Status: constants.BudgetStatusActive})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Delete(ctx, model.Actor{UserID: 1}, sheet.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("error = %v, want ErrInvalidState", err)
	}
}

func TestBudgetServiceAdjustCreatesPendingAdjustment(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditService(newFakeAuditRepo(), testLogger())
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeItemRepo(), newFakeAdjustmentRepo(), audit, nil, testLogger())

	sheet, err := svc.Create(ctx, model.Actor{UserID: 1}, dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	adjustment, err := svc.Adjust(ctx, model.Actor{UserID: 2}, sheet.ID, dto.AdjustBudgetRequest{TotalAmount: 1500, Reason: "增加主材预算"})
	if err != nil {
		t.Fatalf("adjust: %v", err)
	}
	if adjustment.Status != constants.AdjustmentStatusPending {
		t.Fatalf("adjustment status = %q, want Pending", adjustment.Status)
	}
	if adjustment.ProposedAmount != 1500 || adjustment.BudgetVersion != 1 {
		t.Fatalf("unexpected adjustment: proposed=%v version=%d", adjustment.ProposedAmount, adjustment.BudgetVersion)
	}

	// 审批前总额与版本不变。
	got, err := svc.Get(ctx, sheet.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TotalAmount != 1000 || got.Version != 1 {
		t.Fatalf("sheet changed before approval: total=%v version=%d", got.TotalAmount, got.Version)
	}
}

func TestBudgetServiceAdjustSinglePending(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditService(newFakeAuditRepo(), testLogger())
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeItemRepo(), newFakeAdjustmentRepo(), audit, nil, testLogger())

	sheet, err := svc.Create(ctx, model.Actor{UserID: 1}, dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Adjust(ctx, model.Actor{UserID: 2}, sheet.ID, dto.AdjustBudgetRequest{TotalAmount: 1500, Reason: "第一次"}); err != nil {
		t.Fatalf("first adjust: %v", err)
	}
	if _, err := svc.Adjust(ctx, model.Actor{UserID: 2}, sheet.ID, dto.AdjustBudgetRequest{TotalAmount: 1800, Reason: "第二次"}); !errors.Is(err, ErrPendingAdjustmentExists) {
		t.Fatalf("error = %v, want ErrPendingAdjustmentExists", err)
	}
}

func TestBudgetServiceUpdateTotalAmountGoesThroughApproval(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditService(newFakeAuditRepo(), testLogger())
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeItemRepo(), newFakeAdjustmentRepo(), audit, nil, testLogger())

	sheet, err := svc.Create(ctx, model.Actor{UserID: 1}, dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	updated, adjustment, err := svc.Update(ctx, model.Actor{UserID: 2}, sheet.ID, dto.UpdateBudgetRequest{
		Name:         "预算v2",
		TotalAmount:  1500,
		AdjustReason: "更新入口调整",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if adjustment == nil {
		t.Fatalf("expected pending adjustment to be created")
	}
	if adjustment.Status != constants.AdjustmentStatusPending || adjustment.ProposedAmount != 1500 {
		t.Fatalf("unexpected adjustment: %+v", adjustment)
	}
	// 名称直接生效，总额保持原值等待审批。
	if updated.Name != "预算v2" || updated.TotalAmount != 1000 {
		t.Fatalf("unexpected sheet: name=%q total=%v", updated.Name, updated.TotalAmount)
	}
}
