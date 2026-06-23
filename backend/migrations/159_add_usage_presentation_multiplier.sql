ALTER TABLE groups
  ADD COLUMN IF NOT EXISTS usage_multiplier_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS usage_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0;

ALTER TABLE usage_logs
  ADD COLUMN IF NOT EXISTS presentation_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0;

COMMENT ON COLUMN groups.usage_multiplier_enabled IS 'Enable user/admin presentation multiplier for new usage rows';
COMMENT ON COLUMN groups.usage_multiplier IS 'Presentation multiplier used when enabled and usage reaches threshold';
COMMENT ON COLUMN usage_logs.presentation_multiplier IS 'Per-request presentation multiplier snapshot for non-super-admin views';
