CREATE TABLE IF NOT EXISTS budget_adjustments (
    id BIGSERIAL PRIMARY KEY,
    budget_sheet_id BIGINT NOT NULL REFERENCES budget_sheets(id),
    proposed_amount DOUBLE PRECISION NOT NULL,
    reason VARCHAR(512) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'Pending',
    budget_version INT NOT NULL,
    submitted_by_id BIGINT NOT NULL,
    reviewed_by_id BIGINT,
    review_comment VARCHAR(512) NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_budget_adjustments_sheet_id ON budget_adjustments(budget_sheet_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_budget_adjustments_pending ON budget_adjustments(budget_sheet_id) WHERE status = 'Pending';
