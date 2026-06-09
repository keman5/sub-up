ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS model_policy_mode VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model_policy_model VARCHAR(100) NOT NULL DEFAULT '';

COMMENT ON COLUMN groups.model_policy_mode IS '分组级模型策略：空=不限制，force=强制改写为指定模型';
COMMENT ON COLUMN groups.model_policy_model IS '分组级模型策略使用的目标模型 ID';
