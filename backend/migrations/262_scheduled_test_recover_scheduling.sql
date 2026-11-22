-- Explicit opt-in: keep testing disabled scheduling and enable it only after confirmation probes.
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS recover_scheduling BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS recovery_probe_interval_seconds INTEGER NOT NULL DEFAULT 3 CHECK (recovery_probe_interval_seconds BETWEEN 1 AND 3600);
