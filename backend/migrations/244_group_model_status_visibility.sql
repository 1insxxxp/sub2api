-- 244: configure models shown on the per-group model status monitor.
-- This is display-only and must remain independent from model_allowlist.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS model_status_visibility JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.model_status_visibility IS
    '分组模型监控展示范围：{"enabled":bool,"models":string[]}；仅影响模型状态展示，不限制实际请求';
