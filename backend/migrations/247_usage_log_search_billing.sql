-- Additive defaults preserve historical usage and avoid touching old balances.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS search_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS search_cost NUMERIC(20,10) NOT NULL DEFAULT 0;
COMMENT ON COLUMN usage_logs.search_count IS 'Observed billable search queries; repeated SSE snapshots are deduplicated';
COMMENT ON COLUMN usage_logs.search_cost IS 'Search surcharge in USD before the group/user multiplier';
