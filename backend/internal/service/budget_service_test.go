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
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeItemRepo(), audit, nil, testLogger())

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
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeItemRepo(), audit, nil, testLogger())

	sheet, err := svc.Create(ctx, model.Actor{UserID: 1}, dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000, Status: constants.BudgetStatusActive})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Delete(ctx, model.Actor{UserID: 1}, sheet.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("error = %v, want ErrInvalidState", err)
	}
}

func TestBudgetServiceUpdateRejectsDirectTotalChange(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditService(newFakeAuditRepo(), testLogger())
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeItemRepo(), audit, nil, testLogger())

	sheet, err := svc.Create(ctx, model.Actor{UserID: 1}, dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = svc.Update(ctx, model.Actor{UserID: 1}, sheet.ID, dto.UpdateBudgetRequest{TotalAmount: 1500})
	if !errors.Is(err, ErrForbiddenTransition) {
		t.Fatalf("error = %v, want ErrForbiddenTransition", err)
	}
}

func TestBudgetServiceUpdateBumpsVersion(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditService(newFakeAuditRepo(), testLogger())
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeItemRepo(), audit, nil, testLogger())

	sheet, err := svc.Create(ctx, model.Actor{UserID: 1}, dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	updated, err := svc.Update(ctx, model.Actor{UserID: 1}, sheet.ID, dto.UpdateBudgetRequest{Status: constants.BudgetStatusActive})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("version = %d, want 2", updated.Version)
	}
	if updated.TotalAmount != 1000 {
		t.Fatalf("total = %v, want 1000", updated.TotalAmount)
	}
}
