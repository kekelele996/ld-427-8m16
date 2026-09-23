package service

import (
	"context"
	"errors"
	"testing"

	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/dto"
	"github.com/renovation/renovation-budget-api/internal/model"
)

func newAdjustmentServiceWithBudget() (*BudgetAdjustmentService, *BudgetService, *fakeBudgetRepo, *fakeBudgetAdjustmentRepo) {
	audit := NewAuditService(newFakeAuditRepo(), testLogger())
	budgetRepo := newFakeBudgetRepo()
	adjustRepo := newFakeBudgetAdjustmentRepo()
	budgetSvc := NewBudgetService(budgetRepo, newFakeItemRepo(), audit, nil, testLogger())
	adjustSvc := NewBudgetAdjustmentService(adjustRepo, budgetRepo, audit, nil, testLogger())
	return adjustSvc, budgetSvc, budgetRepo, adjustRepo
}

func projectManager() model.Actor {
	return model.Actor{UserID: 10, Username: "project", Role: constants.RoleProjectManager}
}

func financeManager() model.Actor {
	return model.Actor{UserID: 20, Username: "finance", Role: constants.RoleFinanceManager}
}

func TestBudgetAdjustmentSubmitApproveFlow(t *testing.T) {
	ctx := context.Background()
	adjustSvc, budgetSvc, _, _ := newAdjustmentServiceWithBudget()

	sheet, err := budgetSvc.Create(ctx, projectManager(), dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})
	if err != nil {
		t.Fatalf("create budget: %v", err)
	}

	// 提交时不改动总额与版本。
	adj, err := adjustSvc.Submit(ctx, projectManager(), sheet.ID, dto.CreateBudgetAdjustmentRequest{TotalAmount: 1500, Reason: "增加主材预算"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if adj.Status != constants.BudgetAdjustmentStatusPending || adj.BaseVersion != 1 {
		t.Fatalf("unexpected adjustment: %+v", adj)
	}
	gotSheet, _ := budgetSvc.Get(ctx, sheet.ID)
	if gotSheet.TotalAmount != 1000 || gotSheet.Version != 1 || gotSheet.AvailableAmount != 1000 {
		t.Fatalf("budget changed before approval: %+v", gotSheet)
	}

	// 财务经理审批通过后金额生效、余额重算、版本加一，并保留审批信息。
	approved, err := adjustSvc.Approve(ctx, financeManager(), adj.ID, dto.ApproveBudgetAdjustmentRequest{ApprovalComment: "同意"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != constants.BudgetAdjustmentStatusApproved || approved.ApprovedByID == nil {
		t.Fatalf("unexpected approved adjustment: %+v", approved)
	}
	if approved.ApprovalComment != "同意" || approved.ApprovedAt == nil {
		t.Fatalf("approval info not retained: %+v", approved)
	}
	gotSheet, _ = budgetSvc.Get(ctx, sheet.ID)
	if gotSheet.TotalAmount != 1500 {
		t.Fatalf("total = %v, want 1500", gotSheet.TotalAmount)
	}
	if gotSheet.Version != 2 {
		t.Fatalf("version = %d, want 2", gotSheet.Version)
	}
	if gotSheet.AvailableAmount != 1500 {
		t.Fatalf("available = %v, want 1500", gotSheet.AvailableAmount)
	}
}

func TestBudgetAdjustmentRejectKeepsBudget(t *testing.T) {
	ctx := context.Background()
	adjustSvc, budgetSvc, _, _ := newAdjustmentServiceWithBudget()
	sheet, _ := budgetSvc.Create(ctx, projectManager(), dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})

	adj, err := adjustSvc.Submit(ctx, projectManager(), sheet.ID, dto.CreateBudgetAdjustmentRequest{TotalAmount: 3000, Reason: "扩项"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	rejected, err := adjustSvc.Reject(ctx, financeManager(), adj.ID, dto.RejectBudgetAdjustmentRequest{ApprovalComment: "依据不足"})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != constants.BudgetAdjustmentStatusRejected || rejected.ApprovedByID == nil {
		t.Fatalf("unexpected rejected adjustment: %+v", rejected)
	}
	gotSheet, _ := budgetSvc.Get(ctx, sheet.ID)
	if gotSheet.TotalAmount != 1000 || gotSheet.Version != 1 || gotSheet.AvailableAmount != 1000 {
		t.Fatalf("budget changed after rejection: %+v", gotSheet)
	}
}

func TestBudgetAdjustmentVersionStale(t *testing.T) {
	ctx := context.Background()
	adjustSvc, budgetSvc, _, _ := newAdjustmentServiceWithBudget()
	sheet, _ := budgetSvc.Create(ctx, projectManager(), dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})

	adj, _ := adjustSvc.Submit(ctx, projectManager(), sheet.ID, dto.CreateBudgetAdjustmentRequest{TotalAmount: 2000, Reason: "加项"})

	// 提交后预算被改过（名称更新导致版本前进）。
	if _, err := budgetSvc.Update(ctx, projectManager(), sheet.ID, dto.UpdateBudgetRequest{Name: "预算v2"}); err != nil {
		t.Fatalf("update budget: %v", err)
	}

	_, err := adjustSvc.Approve(ctx, financeManager(), adj.ID, dto.ApproveBudgetAdjustmentRequest{})
	if !errors.Is(err, ErrVersionStale) {
		t.Fatalf("approve error = %v, want ErrVersionStale", err)
	}
	stale, _ := adjustSvc.Get(ctx, adj.ID)
	if stale.Status != constants.BudgetAdjustmentStatusExpired {
		t.Fatalf("status = %s, want Expired", stale.Status)
	}
	if stale.ProposedAmount != 2000 || stale.ApprovedByID == nil {
		t.Fatalf("expired adjustment did not retain info: %+v", stale)
	}
	gotSheet, _ := budgetSvc.Get(ctx, sheet.ID)
	if gotSheet.TotalAmount != 1000 {
		t.Fatalf("total = %v, want 1000", gotSheet.TotalAmount)
	}
}

func TestBudgetAdjustmentSinglePendingPerBudget(t *testing.T) {
	ctx := context.Background()
	adjustSvc, budgetSvc, _, _ := newAdjustmentServiceWithBudget()
	sheet, _ := budgetSvc.Create(ctx, projectManager(), dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})

	if _, err := adjustSvc.Submit(ctx, projectManager(), sheet.ID, dto.CreateBudgetAdjustmentRequest{TotalAmount: 1200, Reason: "第一次"}); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	_, err := adjustSvc.Submit(ctx, projectManager(), sheet.ID, dto.CreateBudgetAdjustmentRequest{TotalAmount: 1300, Reason: "第二次"})
	if !errors.Is(err, ErrPendingAdjustment) {
		t.Fatalf("second submit error = %v, want ErrPendingAdjustment", err)
	}

	// 驳回后允许再提一张。
	list, _, _ := adjustSvc.List(ctx, dto.BudgetAdjustmentFilter{BudgetSheetID: sheet.ID})
	pendingID := list[0].ID
	if _, err := adjustSvc.Reject(ctx, financeManager(), pendingID, dto.RejectBudgetAdjustmentRequest{ApprovalComment: "不行"}); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, err := adjustSvc.Submit(ctx, projectManager(), sheet.ID, dto.CreateBudgetAdjustmentRequest{TotalAmount: 1300, Reason: "重新提交"}); err != nil {
		t.Fatalf("resubmit after rejection: %v", err)
	}
}

func TestBudgetAdjustmentRoleRestrictions(t *testing.T) {
	ctx := context.Background()
	adjustSvc, budgetSvc, _, _ := newAdjustmentServiceWithBudget()
	sheet, _ := budgetSvc.Create(ctx, projectManager(), dto.CreateBudgetRequest{ProjectID: "p-1", Name: "预算", TotalAmount: 1000})

	// 会计不能提交调整单。
	if _, err := adjustSvc.Submit(ctx, model.Actor{UserID: 30, Role: constants.RoleAccountant}, sheet.ID, dto.CreateBudgetAdjustmentRequest{TotalAmount: 1200, Reason: "x"}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("accountant submit error = %v, want ErrPermissionDenied", err)
	}
	// 业主不能提交调整单。
	if _, err := adjustSvc.Submit(ctx, model.Actor{UserID: 40, Role: constants.RoleOwner}, sheet.ID, dto.CreateBudgetAdjustmentRequest{TotalAmount: 1200, Reason: "x"}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("owner submit error = %v, want ErrPermissionDenied", err)
	}

	adj, err := adjustSvc.Submit(ctx, projectManager(), sheet.ID, dto.CreateBudgetAdjustmentRequest{TotalAmount: 1200, Reason: "加项"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	// 项目经理不能审批自己提交的调整单；会计与业主也不能审批。
	if _, err := adjustSvc.Approve(ctx, projectManager(), adj.ID, dto.ApproveBudgetAdjustmentRequest{}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("project manager approve error = %v, want ErrPermissionDenied", err)
	}
	if _, err := adjustSvc.Approve(ctx, model.Actor{UserID: 30, Role: constants.RoleAccountant}, adj.ID, dto.ApproveBudgetAdjustmentRequest{}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("accountant approve error = %v, want ErrPermissionDenied", err)
	}
	if _, err := adjustSvc.Approve(ctx, model.Actor{UserID: 40, Role: constants.RoleOwner}, adj.ID, dto.ApproveBudgetAdjustmentRequest{}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("owner approve error = %v, want ErrPermissionDenied", err)
	}
}
