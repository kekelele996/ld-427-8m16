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

// AdjustmentService 预算调整单业务逻辑。
type AdjustmentService struct {
	repo       repository.AdjustmentRepository
	budgetRepo repository.BudgetRepository
	audit      *AuditService
	rdb        *redis.Client
	logger     *slog.Logger
}

// NewAdjustmentService 构造预算调整单服务。
func NewAdjustmentService(repo repository.AdjustmentRepository, budgetRepo repository.BudgetRepository, audit *AuditService, rdb *redis.Client, logger *slog.Logger) *AdjustmentService {
	return &AdjustmentService{repo: repo, budgetRepo: budgetRepo, audit: audit, rdb: rdb, logger: logger}
}

// submitAdjustment 校验并创建待审批的预算调整单。同一份预算表同一时间只允许一张待审单。
// 预算服务的 Adjust/Update 入口复用该helper，保证所有总额变更都进入审批流程。
func submitAdjustment(ctx context.Context, repo repository.AdjustmentRepository, actor model.Actor, sheet *model.BudgetSheet, proposedAmount float64, reason string) (*model.BudgetAdjustment, error) {
	if sheet.Status == constants.BudgetStatusArchived {
		return nil, fmt.Errorf("adjust budget sheet %d: %w", sheet.ID, ErrInvalidState)
	}
	if _, err := repo.FindPendingByBudgetID(ctx, sheet.ID); err == nil {
		return nil, fmt.Errorf("adjust budget sheet %d: %w", sheet.ID, ErrPendingAdjustmentExists)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check pending adjustment for budget sheet %d: %w", sheet.ID, err)
	}
	adjustment := &model.BudgetAdjustment{
		BudgetSheetID:  sheet.ID,
		ProposedAmount: proposedAmount,
		Reason:         reason,
		Status:         constants.AdjustmentStatusPending,
		BudgetVersion:  sheet.Version,
		SubmittedByID:  actor.UserID,
	}
	if err := repo.Create(ctx, adjustment); err != nil {
		return nil, fmt.Errorf("create budget adjustment for sheet %d: %w", sheet.ID, err)
	}
	return adjustment, nil
}

// Submit 提交预算调整单，金额在审批通过前不生效。
func (s *AdjustmentService) Submit(ctx context.Context, actor model.Actor, budgetSheetID uint, req dto.SubmitAdjustmentRequest) (*model.BudgetAdjustment, error) {
	sheet, err := s.budgetRepo.FindByID(ctx, budgetSheetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget sheet %d: %w", budgetSheetID, err)
	}
	adjustment, err := submitAdjustment(ctx, s.repo, actor, sheet, req.ProposedAmount, req.Reason)
	if err != nil {
		return nil, err
	}
	s.audit.Record(ctx, actor, "budget_adjust_submit", "budget_adjustment", adjustment.ID,
		fmt.Sprintf("budget_sheet_id=%d proposed=%.2f version=%d reason=%s", sheet.ID, adjustment.ProposedAmount, adjustment.BudgetVersion, adjustment.Reason))
	return adjustment, nil
}

// List 查询预算表的调整单列表。
func (s *AdjustmentService) List(ctx context.Context, budgetSheetID uint) ([]model.BudgetAdjustment, error) {
	if _, err := s.budgetRepo.FindByID(ctx, budgetSheetID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget sheet %d: %w", budgetSheetID, err)
	}
	adjustments, err := s.repo.ListByBudgetID(ctx, budgetSheetID)
	if err != nil {
		return nil, fmt.Errorf("list adjustments for budget sheet %d: %w", budgetSheetID, err)
	}
	return adjustments, nil
}

// Get 获取预算调整单详情。
func (s *AdjustmentService) Get(ctx context.Context, id uint) (*model.BudgetAdjustment, error) {
	adjustment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget adjustment %d: %w", id, err)
	}
	return adjustment, nil
}

// Approve 批准预算调整单：总额生效、可用余额重算、预算版本加一。
// 提交后预算版本若已变化，调整单无法批准并提示版本过期。
func (s *AdjustmentService) Approve(ctx context.Context, actor model.Actor, id uint, req dto.ApproveAdjustmentRequest) (*model.BudgetAdjustment, error) {
	adjustment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget adjustment %d: %w", id, err)
	}
	if adjustment.Status != constants.AdjustmentStatusPending {
		return nil, fmt.Errorf("approve budget adjustment %d: %w", id, ErrInvalidState)
	}
	sheet, err := s.budgetRepo.FindByID(ctx, adjustment.BudgetSheetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget sheet %d: %w", adjustment.BudgetSheetID, err)
	}
	if sheet.Status == constants.BudgetStatusArchived {
		return nil, fmt.Errorf("approve budget adjustment %d: %w", id, ErrInvalidState)
	}
	if sheet.Version != adjustment.BudgetVersion {
		return nil, fmt.Errorf("approve budget adjustment %d: %w", id, ErrStaleVersion)
	}

	sheet.TotalAmount = adjustment.ProposedAmount
	sheet.Version++
	sheet.AvailableAmount = CalculateAvailable(sheet.TotalAmount, sheet.SpentAmount, sheet.FrozenAmount)

	now := time.Now()
	adjustment.Status = constants.AdjustmentStatusApproved
	adjustment.ReviewedByID = &actor.UserID
	adjustment.ReviewComment = req.ReviewComment
	adjustment.ReviewedAt = &now

	if err := s.budgetRepo.Update(ctx, sheet); err != nil {
		return nil, fmt.Errorf("apply budget adjustment %d: %w", id, err)
	}
	if err := s.repo.Update(ctx, adjustment); err != nil {
		return nil, fmt.Errorf("approve budget adjustment %d: %w", id, err)
	}
	s.invalidateBudget(ctx, sheet.ID)
	s.audit.Record(ctx, actor, "budget_adjust_approve", "budget_adjustment", id,
		fmt.Sprintf("budget_sheet_id=%d total=%.2f version=%d comment=%s", sheet.ID, sheet.TotalAmount, sheet.Version, req.ReviewComment))
	return adjustment, nil
}

// Reject 驳回预算调整单，预算金额不变。
func (s *AdjustmentService) Reject(ctx context.Context, actor model.Actor, id uint, req dto.RejectAdjustmentRequest) (*model.BudgetAdjustment, error) {
	adjustment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget adjustment %d: %w", id, err)
	}
	if adjustment.Status != constants.AdjustmentStatusPending {
		return nil, fmt.Errorf("reject budget adjustment %d: %w", id, ErrInvalidState)
	}

	now := time.Now()
	adjustment.Status = constants.AdjustmentStatusRejected
	adjustment.ReviewedByID = &actor.UserID
	adjustment.ReviewComment = req.ReviewComment
	adjustment.ReviewedAt = &now

	if err := s.repo.Update(ctx, adjustment); err != nil {
		return nil, fmt.Errorf("reject budget adjustment %d: %w", id, err)
	}
	s.audit.Record(ctx, actor, "budget_adjust_reject", "budget_adjustment", id,
		fmt.Sprintf("budget_sheet_id=%d proposed=%.2f comment=%s", adjustment.BudgetSheetID, adjustment.ProposedAmount, req.ReviewComment))
	return adjustment, nil
}

func (s *AdjustmentService) invalidateBudget(ctx context.Context, budgetSheetID uint) {
	if s.rdb == nil {
		return
	}
	if err := s.rdb.Del(ctx, budgetSnapshotKey(budgetSheetID)).Err(); err != nil {
		s.logger.Warn("invalidate budget snapshot failed", slog.Uint64("budget_id", uint64(budgetSheetID)), slog.String("error", err.Error()))
	}
}
