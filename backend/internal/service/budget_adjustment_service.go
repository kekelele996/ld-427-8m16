package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/dto"
	"github.com/renovation/renovation-budget-api/internal/model"
	"github.com/renovation/renovation-budget-api/internal/repository"
)

// BudgetAdjustmentService 预算调整单业务逻辑。
//
// 预算总额的任何变更都必须经调整单审批：项目经理提交、财务经理审批通过后
// 金额才生效，可用余额同步重算，预算版本号加一。会计与业主不参与审批。
type BudgetAdjustmentService struct {
	repo       repository.BudgetAdjustmentRepository
	budgetRepo repository.BudgetRepository
	audit      *AuditService
	rdb        *redis.Client
	logger     *slog.Logger
}

// NewBudgetAdjustmentService 构造预算调整单服务。
func NewBudgetAdjustmentService(repo repository.BudgetAdjustmentRepository, budgetRepo repository.BudgetRepository, audit *AuditService, rdb *redis.Client, logger *slog.Logger) *BudgetAdjustmentService {
	return &BudgetAdjustmentService{repo: repo, budgetRepo: budgetRepo, audit: audit, rdb: rdb, logger: logger}
}

// Submit 项目经理基于预算当前版本提交调整单（拟调金额与原因），不直接改总额。
// 同一份预算只允许存在一张待审单；归档预算不允许调整。
func (s *BudgetAdjustmentService) Submit(ctx context.Context, actor model.Actor, budgetSheetID uint, req dto.CreateBudgetAdjustmentRequest) (*model.BudgetAdjustment, error) {
	if actor.Role != constants.RoleAdmin && actor.Role != constants.RoleProjectManager {
		return nil, fmt.Errorf("submit budget adjustment for sheet %d: %w", budgetSheetID, ErrPermissionDenied)
	}
	sheet, err := s.budgetRepo.FindByID(ctx, budgetSheetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget sheet %d: %w", budgetSheetID, err)
	}
	if sheet.Status == constants.BudgetStatusArchived {
		return nil, fmt.Errorf("submit budget adjustment for archived sheet %d: %w", budgetSheetID, ErrInvalidState)
	}
	if pending, err := s.repo.FindPendingByBudgetID(ctx, budgetSheetID); err == nil && pending != nil {
		return nil, fmt.Errorf("submit budget adjustment for sheet %d: %w", budgetSheetID, ErrPendingAdjustment)
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check pending budget adjustment for sheet %d: %w", budgetSheetID, err)
	}

	adjustment := &model.BudgetAdjustment{
		BudgetSheetID:  sheet.ID,
		ProposedAmount: req.TotalAmount,
		Reason:         req.Reason,
		BaseVersion:    sheet.Version,
		Status:         constants.BudgetAdjustmentStatusPending,
		ApplicantID:    actor.UserID,
	}
	if err := s.repo.Create(ctx, adjustment); err != nil {
		return nil, fmt.Errorf("create budget adjustment: %w", err)
	}
	s.audit.Record(ctx, actor, "budget_adjust_submit", "budget_adjustment", adjustment.ID,
		fmt.Sprintf("budget_sheet_id=%d proposed=%.2f base_version=%d reason=%s", sheet.ID, adjustment.ProposedAmount, adjustment.BaseVersion, adjustment.Reason))
	return adjustment, nil
}

// Get 获取预算调整单。
func (s *BudgetAdjustmentService) Get(ctx context.Context, id uint) (*model.BudgetAdjustment, error) {
	adjustment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget adjustment %d: %w", id, err)
	}
	return adjustment, nil
}

// List 分页查询预算调整单。
func (s *BudgetAdjustmentService) List(ctx context.Context, filter dto.BudgetAdjustmentFilter) ([]model.BudgetAdjustment, int64, error) {
	adjustments, total, err := s.repo.List(ctx, repository.BudgetAdjustmentListFilter{
		BudgetSheetID: filter.BudgetSheetID,
		Status:        string(filter.Status),
		Page:          filter.Page,
		PageSize:      filter.PageSize,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list budget adjustments: %w", err)
	}
	return adjustments, total, nil
}

// Approve 财务经理审批通过调整单：金额生效、可用余额重算、预算版本加一。
// 若提交后预算已被改动（版本号前进），调整单置为 Expired 且无法批准，提示版本过期。
func (s *BudgetAdjustmentService) Approve(ctx context.Context, actor model.Actor, id uint, req dto.ApproveBudgetAdjustmentRequest) (*model.BudgetAdjustment, error) {
	if actor.Role != constants.RoleAdmin && actor.Role != constants.RoleFinanceManager {
		return nil, fmt.Errorf("approve budget adjustment %d: %w", id, ErrPermissionDenied)
	}
	adjustment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget adjustment %d: %w", id, err)
	}
	if adjustment.Status != constants.BudgetAdjustmentStatusPending {
		return nil, fmt.Errorf("approve budget adjustment %d in status %s: %w", id, adjustment.Status, ErrInvalidState)
	}
	sheet, err := s.budgetRepo.FindByID(ctx, adjustment.BudgetSheetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget sheet %d: %w", adjustment.BudgetSheetID, err)
	}

	// 乐观版本校验：提交后预算版本前进则该单过期，不能批准。
	if sheet.Version != adjustment.BaseVersion {
		adjustment.Status = constants.BudgetAdjustmentStatusExpired
		adjustment.ApprovedByID = &actor.UserID
		adjustment.ApprovalComment = req.ApprovalComment
		now := time.Now()
		adjustment.ApprovedAt = &now
		if updateErr := s.repo.Update(ctx, adjustment); updateErr != nil {
			return nil, fmt.Errorf("mark stale budget adjustment %d expired: %w", id, updateErr)
		}
		s.audit.Record(ctx, actor, "budget_adjust_expire", "budget_adjustment", id,
			fmt.Sprintf("budget_sheet_id=%d base_version=%d current_version=%d", sheet.ID, adjustment.BaseVersion, sheet.Version))
		return nil, fmt.Errorf("approve budget adjustment %d against version %d/%d: %w", id, adjustment.BaseVersion, sheet.Version, ErrVersionStale)
	}

	now := time.Now()
	adjustment.Status = constants.BudgetAdjustmentStatusApproved
	adjustment.ApprovedByID = &actor.UserID
	adjustment.ApprovalComment = req.ApprovalComment
	adjustment.ApprovedAt = &now

	sheet.TotalAmount = adjustment.ProposedAmount
	sheet.Version++
	sheet.AvailableAmount = CalculateAvailable(sheet.TotalAmount, sheet.SpentAmount, sheet.FrozenAmount)

	if err := s.repo.Update(ctx, adjustment); err != nil {
		return nil, fmt.Errorf("approve budget adjustment %d: %w", id, err)
	}
	if err := s.budgetRepo.Update(ctx, sheet); err != nil {
		return nil, fmt.Errorf("apply budget adjustment %d to sheet %d: %w", id, sheet.ID, err)
	}
	s.invalidateSnapshot(ctx, sheet.ID)
	s.audit.Record(ctx, actor, "budget_adjust_approve", "budget_adjustment", id,
		fmt.Sprintf("budget_sheet_id=%d total=%.2f version=%d comment=%s", sheet.ID, sheet.TotalAmount, sheet.Version, req.ApprovalComment))
	return adjustment, nil
}

// Reject 财务经理驳回调整单。驳回不改变预算金额与版本。
func (s *BudgetAdjustmentService) Reject(ctx context.Context, actor model.Actor, id uint, req dto.RejectBudgetAdjustmentRequest) (*model.BudgetAdjustment, error) {
	if actor.Role != constants.RoleAdmin && actor.Role != constants.RoleFinanceManager {
		return nil, fmt.Errorf("reject budget adjustment %d: %w", id, ErrPermissionDenied)
	}
	adjustment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget adjustment %d: %w", id, err)
	}
	if adjustment.Status != constants.BudgetAdjustmentStatusPending {
		return nil, fmt.Errorf("reject budget adjustment %d in status %s: %w", id, adjustment.Status, ErrInvalidState)
	}
	now := time.Now()
	adjustment.Status = constants.BudgetAdjustmentStatusRejected
	adjustment.ApprovedByID = &actor.UserID
	adjustment.ApprovalComment = req.ApprovalComment
	adjustment.ApprovedAt = &now
	if err := s.repo.Update(ctx, adjustment); err != nil {
		return nil, fmt.Errorf("reject budget adjustment %d: %w", id, err)
	}
	s.audit.Record(ctx, actor, "budget_adjust_reject", "budget_adjustment", id,
		fmt.Sprintf("budget_sheet_id=%d proposed=%.2f comment=%s", adjustment.BudgetSheetID, adjustment.ProposedAmount, req.ApprovalComment))
	return adjustment, nil
}

func (s *BudgetAdjustmentService) invalidateSnapshot(ctx context.Context, budgetSheetID uint) {
	if s.rdb == nil {
		return
	}
	if err := s.rdb.Del(ctx, budgetSnapshotKey(budgetSheetID)).Err(); err != nil {
		s.logger.Warn("invalidate budget snapshot failed", slog.Uint64("budget_id", uint64(budgetSheetID)), slog.String("error", err.Error()))
	}
}
