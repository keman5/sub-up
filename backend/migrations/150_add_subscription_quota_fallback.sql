ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS quota_fallback_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS quota_fallback_model VARCHAR(100) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_groups_quota_fallback_group_id
    ON groups(quota_fallback_group_id)
    WHERE deleted_at IS NULL AND quota_fallback_group_id IS NOT NULL;

COMMENT ON COLUMN groups.quota_fallback_group_id IS '订阅额度耗尽后自动切换使用的附属套餐分组 ID';
COMMENT ON COLUMN groups.quota_fallback_model IS '订阅额度耗尽切换附属套餐时强制使用的模型 ID';
