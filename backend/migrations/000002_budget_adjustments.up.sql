CREATE TABLE IF NOT EXISTS budget_adjustments (
    id BIGSERIAL PRIMARY KEY,
    budget_sheet_id BIGINT NOT NULL REFERENCES budget_sheets(id) ON DELETE CASCADE,
    proposed_amount DOUBLE PRECISION NOT NULL,
    reason VARCHAR(512) NOT NULL DEFAULT '',
    base_version INT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'Pending',
    applicant_id BIGINT NOT NULL,
    approved_by_id BIGINT,
    approval_comment VARCHAR(512) NOT NULL DEFAULT '',
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_budget_adjustments_sheet_id ON budget_adjustments(budget_sheet_id);
CREATE INDEX IF NOT EXISTS idx_budget_adjustments_status ON budget_adjustments(status);
-- 同一份预算只允许存在一张待审（Pending）调整单。
CREATE UNIQUE INDEX IF NOT EXISTS uq_budget_adjustments_one_pending
    ON budget_adjustments(budget_sheet_id)
    WHERE status = 'Pending';
