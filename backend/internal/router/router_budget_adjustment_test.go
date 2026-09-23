package router_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/renovation/renovation-budget-api/internal/auth"
	"github.com/renovation/renovation-budget-api/internal/config"
	"github.com/renovation/renovation-budget-api/internal/constants"
	"github.com/renovation/renovation-budget-api/internal/handler"
	"github.com/renovation/renovation-budget-api/internal/model"
	"github.com/renovation/renovation-budget-api/internal/repository"
	"github.com/renovation/renovation-budget-api/internal/router"
	"github.com/renovation/renovation-budget-api/internal/service"
)

const jwtSecret = "test-secret"

type apiBody struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupEngine(t *testing.T) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Role{}, &model.User{}, &model.BudgetSheet{}, &model.BudgetItem{},
		&model.BudgetAdjustment{}, &model.ExpenseRecord{}, &model.Supplier{},
		&model.Reconciliation{}, &model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	budgetRepo := repository.NewBudgetRepository(db)
	adjustRepo := repository.NewBudgetAdjustmentRepository(db)
	itemRepo := repository.NewItemRepository(db)
	expenseRepo := repository.NewExpenseRepository(db)
	supplierRepo := repository.NewSupplierRepository(db)
	reconRepo := repository.NewReconciliationRepository(db)

	auditService := service.NewAuditService(auditRepo, logger)
	authService := service.NewAuthService(userRepo, &config.Config{JWTSecret: jwtSecret}, logger)
	budgetService := service.NewBudgetService(budgetRepo, itemRepo, auditService, nil, logger)
	adjustService := service.NewBudgetAdjustmentService(adjustRepo, budgetRepo, auditService, nil, logger)
	itemService := service.NewItemService(itemRepo, budgetRepo, auditService, nil, logger)
	expenseService := service.NewExpenseService(expenseRepo, itemRepo, budgetRepo, auditService, nil, logger)
	supplierService := service.NewSupplierService(supplierRepo, auditService, logger)
	reconService := service.NewReconciliationService(reconRepo, auditService, logger)

	// 指向不可达地址：限流中间件在 Redis 不可用时放行。
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})

	engine := router.New(router.Dependencies{
		Config:                  &config.Config{JWTSecret: jwtSecret, RateLimitMax: 1000, RateLimitWindowSeconds: 60},
		Redis:                   rdb,
		Logger:                  logger,
		AuditRepo:               auditRepo,
		AuthHandler:             handler.NewAuthHandler(authService, logger),
		AuditHandler:            handler.NewAuditHandler(auditService, logger),
		BudgetHandler:           handler.NewBudgetHandler(budgetService, adjustService, logger),
		BudgetAdjustmentHandler: handler.NewBudgetAdjustmentHandler(adjustService, logger),
		ItemHandler:             handler.NewItemHandler(itemService, logger),
		ExpenseHandler:          handler.NewExpenseHandler(expenseService, logger),
		SupplierHandler:         handler.NewSupplierHandler(supplierService, logger),
		ReconciliationHandler:   handler.NewReconciliationHandler(reconService, logger),
	})
	return engine
}

func token(t *testing.T, userID uint, username string, role constants.RoleName) string {
	t.Helper()
	signed, err := auth.GenerateToken(jwtSecret, userID, username, role, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return signed
}

func doJSON(t *testing.T, h http.Handler, method, path, tokenString string, payload any) (int, apiBody) {
	t.Helper()
	var reader io.Reader
	if payload != nil {
		raw, _ := json.Marshal(payload)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if tokenString != "" {
		req.Header.Set("Authorization", "Bearer "+tokenString)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body apiBody
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, body
}

func mustDataID(t *testing.T, raw json.RawMessage) uint {
	t.Helper()
	var m struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	return m.ID
}

func TestBudgetAdjustmentApprovalFlowHTTP(t *testing.T) {
	h := setupEngine(t)
	pm := token(t, 10, "project", constants.RoleProjectManager)
	fm := token(t, 20, "finance", constants.RoleFinanceManager)

	_, created := doJSON(t, h, http.MethodPost, "/api/v1/budgets", pm, map[string]any{
		"project_id": "p-1", "name": "整屋", "total_amount": 1000,
	})
	budgetID := mustDataID(t, created.Data)

	// 提交调整单不立即改总额。
	status, body := doJSON(t, h, http.MethodPost, "/api/v1/budgets/"+itoa(budgetID)+"/adjustments", pm, map[string]any{
		"total_amount": 1500, "reason": "主材涨价",
	})
	if status != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", status, body.Message)
	}
	adjID := mustDataID(t, body.Data)

	status, got := doJSON(t, h, http.MethodGet, "/api/v1/budgets/"+itoa(budgetID), pm, nil)
	if status != http.StatusOK {
		t.Fatalf("get budget: %d", status)
	}
	var sheet model.BudgetSheet
	if err := json.Unmarshal(got.Data, &sheet); err != nil {
		t.Fatalf("decode sheet: %v", err)
	}
	if sheet.TotalAmount != 1000 || sheet.Version != 1 {
		t.Fatalf("budget changed before approval: total=%v version=%d", sheet.TotalAmount, sheet.Version)
	}

	// 财务经理通过后金额生效、余额重算、版本加一。
	status, body = doJSON(t, h, http.MethodPost, "/api/v1/budget-adjustments/"+itoa(adjID)+"/approve", fm, map[string]any{
		"approval_comment": "同意",
	})
	if status != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", status, body.Message)
	}
	var adj model.BudgetAdjustment
	if err := json.Unmarshal(body.Data, &adj); err != nil {
		t.Fatalf("decode adjustment: %v", err)
	}
	if adj.Status != constants.BudgetAdjustmentStatusApproved || adj.ApprovedByID == nil || adj.ApprovalComment != "同意" {
		t.Fatalf("approval info not retained: %+v", adj)
	}

	_, got = doJSON(t, h, http.MethodGet, "/api/v1/budgets/"+itoa(budgetID), fm, nil)
	_ = json.Unmarshal(got.Data, &sheet)
	if sheet.TotalAmount != 1500 || sheet.AvailableAmount != 1500 || sheet.Version != 2 {
		t.Fatalf("budget not applied: total=%v available=%v version=%d", sheet.TotalAmount, sheet.AvailableAmount, sheet.Version)
	}
}

func TestBudgetAdjustmentStaleVersionHTTP(t *testing.T) {
	h := setupEngine(t)
	pm := token(t, 10, "project", constants.RoleProjectManager)
	fm := token(t, 20, "finance", constants.RoleFinanceManager)

	_, created := doJSON(t, h, http.MethodPost, "/api/v1/budgets", pm, map[string]any{
		"project_id": "p-2", "name": "整屋", "total_amount": 1000,
	})
	budgetID := mustDataID(t, created.Data)

	_, body := doJSON(t, h, http.MethodPost, "/api/v1/budgets/"+itoa(budgetID)+"/adjustments", pm, map[string]any{
		"total_amount": 2000, "reason": "扩项",
	})
	adjID := mustDataID(t, body.Data)

	// 提交后预算被改过（名称更新，版本前进）。
	if status, body := doJSON(t, h, http.MethodPut, "/api/v1/budgets/"+itoa(budgetID), pm, map[string]any{
		"name": "整屋v2",
	}); status != http.StatusOK {
		t.Fatalf("update budget status=%d msg=%s", status, body.Message)
	}

	// 批准被拒绝并提示版本过期。
	status, body := doJSON(t, h, http.MethodPost, "/api/v1/budget-adjustments/"+itoa(adjID)+"/approve", fm, map[string]any{})
	if status != http.StatusConflict {
		t.Fatalf("approve stale status=%d, want 409", status)
	}
	_, got := doJSON(t, h, http.MethodGet, "/api/v1/budget-adjustments/"+itoa(adjID), fm, nil)
	var adj model.BudgetAdjustment
	_ = json.Unmarshal(got.Data, &adj)
	if adj.Status != constants.BudgetAdjustmentStatusExpired {
		t.Fatalf("status = %s, want Expired", adj.Status)
	}
}

func TestBudgetAdjustmentRBACAndSinglePendingHTTP(t *testing.T) {
	h := setupEngine(t)
	pm := token(t, 10, "project", constants.RoleProjectManager)
	fm := token(t, 20, "finance", constants.RoleFinanceManager)
	accountant := token(t, 30, "accountant", constants.RoleAccountant)
	owner := token(t, 40, "owner", constants.RoleOwner)

	_, created := doJSON(t, h, http.MethodPost, "/api/v1/budgets", pm, map[string]any{
		"project_id": "p-3", "name": "整屋", "total_amount": 1000,
	})
	budgetID := mustDataID(t, created.Data)
	submitPath := "/api/v1/budgets/" + itoa(budgetID) + "/adjustments"

	// 会计与业主不能提交。
	for _, tok := range []string{accountant, owner} {
		if status, _ := doJSON(t, h, http.MethodPost, submitPath, tok, map[string]any{"total_amount": 1200, "reason": "x"}); status != http.StatusForbidden {
			t.Fatalf("submit status=%d, want 403", status)
		}
	}

	_, body := doJSON(t, h, http.MethodPost, submitPath, pm, map[string]any{"total_amount": 1200, "reason": "第一次"})
	adjID := mustDataID(t, body.Data)

	// 同一预算只能有一张待审单。
	if status, body := doJSON(t, h, http.MethodPost, submitPath, pm, map[string]any{"total_amount": 1300, "reason": "第二次"}); status != http.StatusConflict {
		t.Fatalf("duplicate pending status=%d msg=%s, want 409", status, body.Message)
	}

	// 项目经理不能审批；会计、业主也不能。
	approvePath := "/api/v1/budget-adjustments/" + itoa(adjID) + "/approve"
	for _, tok := range []string{pm, accountant, owner} {
		if status, _ := doJSON(t, h, http.MethodPost, approvePath, tok, map[string]any{}); status != http.StatusForbidden {
			t.Fatalf("approve status=%d, want 403", status)
		}
	}

	// 财务经理可以驳回；驳回后允许重新提交。
	if status, body := doJSON(t, h, http.MethodPost, "/api/v1/budget-adjustments/"+itoa(adjID)+"/reject", fm, map[string]any{"approval_comment": "不行"}); status != http.StatusOK {
		t.Fatalf("reject status=%d msg=%s", status, body.Message)
	}
	if status, _ := doJSON(t, h, http.MethodPost, submitPath, pm, map[string]any{"total_amount": 1300, "reason": "重新提交"}); status != http.StatusOK {
		t.Fatalf("resubmit status=%d, want 200", status)
	}
}

func TestDirectTotalChangeGuardedHTTP(t *testing.T) {
	h := setupEngine(t)
	pm := token(t, 10, "project", constants.RoleProjectManager)

	_, created := doJSON(t, h, http.MethodPost, "/api/v1/budgets", pm, map[string]any{
		"project_id": "p-4", "name": "整屋", "total_amount": 1000,
	})
	budgetID := mustDataID(t, created.Data)

	// 旧的直接调总额入口（/adjust）现在转入审批门，返回一张待审单而非直接改总额。
	status, body := doJSON(t, h, http.MethodPost, "/api/v1/budgets/"+itoa(budgetID)+"/adjust", pm, map[string]any{
		"total_amount": 1700, "reason": "旧入口加项",
	})
	if status != http.StatusOK {
		t.Fatalf("legacy adjust status=%d msg=%s", status, body.Message)
	}
	var adj model.BudgetAdjustment
	_ = json.Unmarshal(body.Data, &adj)
	if adj.Status != constants.BudgetAdjustmentStatusPending || adj.ProposedAmount != 1700 {
		t.Fatalf("legacy adjust did not create pending adjustment: %+v", adj)
	}

	// PUT 更新入口携带 total_amount 且缺 reason 时拒绝（400）。
	if status, _ := doJSON(t, h, http.MethodPut, "/api/v1/budgets/"+itoa(budgetID), pm, map[string]any{
		"total_amount": 1800,
	}); status != http.StatusBadRequest {
		t.Fatalf("put without reason status=%d, want 400", status)
	}
}

func itoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
