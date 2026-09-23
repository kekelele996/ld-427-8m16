package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/model"
)

// AdjustmentRepository 预算调整单数据访问接口。
type AdjustmentRepository interface {
	Create(ctx context.Context, adjustment *model.BudgetAdjustment) error
	FindByID(ctx context.Context, id uint) (*model.BudgetAdjustment, error)
	FindPendingByBudgetID(ctx context.Context, budgetSheetID uint) (*model.BudgetAdjustment, error)
	ListByBudgetID(ctx context.Context, budgetSheetID uint) ([]model.BudgetAdjustment, error)
	Update(ctx context.Context, adjustment *model.BudgetAdjustment) error
}

type adjustmentRepository struct {
	db *gorm.DB
}

// NewAdjustmentRepository 构造预算调整单仓储。
func NewAdjustmentRepository(db *gorm.DB) AdjustmentRepository {
	return &adjustmentRepository{db: db}
}

func (r *adjustmentRepository) Create(ctx context.Context, adjustment *model.BudgetAdjustment) error {
	if err := r.db.WithContext(ctx).Create(adjustment).Error; err != nil {
		return fmt.Errorf("create budget adjustment: %w", err)
	}
	return nil
}

func (r *adjustmentRepository) FindByID(ctx context.Context, id uint) (*model.BudgetAdjustment, error) {
	var adjustment model.BudgetAdjustment
	if err := r.db.WithContext(ctx).First(&adjustment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find budget adjustment %d: %w", id, err)
	}
	return &adjustment, nil
}

func (r *adjustmentRepository) FindPendingByBudgetID(ctx context.Context, budgetSheetID uint) (*model.BudgetAdjustment, error) {
	var adjustment model.BudgetAdjustment
	err := r.db.WithContext(ctx).
		Where("budget_sheet_id = ? AND status = ?", budgetSheetID, constants.AdjustmentStatusPending).
		Order("id DESC").
		First(&adjustment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find pending adjustment for budget sheet %d: %w", budgetSheetID, err)
	}
	return &adjustment, nil
}

func (r *adjustmentRepository) ListByBudgetID(ctx context.Context, budgetSheetID uint) ([]model.BudgetAdjustment, error) {
	var adjustments []model.BudgetAdjustment
	if err := r.db.WithContext(ctx).
		Where("budget_sheet_id = ?", budgetSheetID).
		Order("id DESC").
		Find(&adjustments).Error; err != nil {
		return nil, fmt.Errorf("list adjustments for budget sheet %d: %w", budgetSheetID, err)
	}
	return adjustments, nil
}

func (r *adjustmentRepository) Update(ctx context.Context, adjustment *model.BudgetAdjustment) error {
	if err := r.db.WithContext(ctx).Save(adjustment).Error; err != nil {
		return fmt.Errorf("update budget adjustment %d: %w", adjustment.ID, err)
	}
	return nil
}
