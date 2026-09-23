package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/model"
)

// BudgetAdjustmentListFilter 预算调整单列表过滤条件。
type BudgetAdjustmentListFilter struct {
	BudgetSheetID uint
	Status        string
	Page          int
	PageSize      int
}

// BudgetAdjustmentRepository 预算调整单数据访问接口。
type BudgetAdjustmentRepository interface {
	Create(ctx context.Context, adjustment *model.BudgetAdjustment) error
	FindByID(ctx context.Context, id uint) (*model.BudgetAdjustment, error)
	List(ctx context.Context, filter BudgetAdjustmentListFilter) ([]model.BudgetAdjustment, int64, error)
	FindPendingByBudgetID(ctx context.Context, budgetSheetID uint) (*model.BudgetAdjustment, error)
	Update(ctx context.Context, adjustment *model.BudgetAdjustment) error
}

type budgetAdjustmentRepository struct {
	db *gorm.DB
}

// NewBudgetAdjustmentRepository 构造预算调整单仓储。
func NewBudgetAdjustmentRepository(db *gorm.DB) BudgetAdjustmentRepository {
	return &budgetAdjustmentRepository{db: db}
}

func (r *budgetAdjustmentRepository) Create(ctx context.Context, adjustment *model.BudgetAdjustment) error {
	if err := r.db.WithContext(ctx).Create(adjustment).Error; err != nil {
		return fmt.Errorf("create budget adjustment: %w", err)
	}
	return nil
}

func (r *budgetAdjustmentRepository) FindByID(ctx context.Context, id uint) (*model.BudgetAdjustment, error) {
	var adjustment model.BudgetAdjustment
	if err := r.db.WithContext(ctx).First(&adjustment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find budget adjustment %d: %w", id, err)
	}
	return &adjustment, nil
}

func (r *budgetAdjustmentRepository) List(ctx context.Context, filter BudgetAdjustmentListFilter) ([]model.BudgetAdjustment, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	q := r.db.WithContext(ctx).Model(&model.BudgetAdjustment{})
	if filter.BudgetSheetID != 0 {
		q = q.Where("budget_sheet_id = ?", filter.BudgetSheetID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count budget adjustments: %w", err)
	}
	var adjustments []model.BudgetAdjustment
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&adjustments).Error; err != nil {
		return nil, 0, fmt.Errorf("list budget adjustments: %w", err)
	}
	return adjustments, total, nil
}

func (r *budgetAdjustmentRepository) FindPendingByBudgetID(ctx context.Context, budgetSheetID uint) (*model.BudgetAdjustment, error) {
	var adjustment model.BudgetAdjustment
	err := r.db.WithContext(ctx).
		Where("budget_sheet_id = ? AND status = ?", budgetSheetID, constants.BudgetAdjustmentStatusPending).
		First(&adjustment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find pending budget adjustment for sheet %d: %w", budgetSheetID, err)
	}
	return &adjustment, nil
}

func (r *budgetAdjustmentRepository) Update(ctx context.Context, adjustment *model.BudgetAdjustment) error {
	if err := r.db.WithContext(ctx).Save(adjustment).Error; err != nil {
		return fmt.Errorf("update budget adjustment %d: %w", adjustment.ID, err)
	}
	return nil
}
