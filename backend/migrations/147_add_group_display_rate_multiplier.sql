ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS display_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0;

COMMENT ON COLUMN groups.display_rate_multiplier IS '用户端展示倍率，仅影响 UI 展示，不参与实际计费';
