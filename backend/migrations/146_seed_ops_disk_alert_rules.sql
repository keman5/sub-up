-- Seed disk usage alert rules for ops monitoring.
INSERT INTO ops_alert_rules (
    name, description, enabled, metric_type, operator, threshold,
    window_minutes, sustained_minutes, severity, notify_email, cooldown_minutes,
    created_at, updated_at
) VALUES (
    '磁盘使用率偏高',
    '当磁盘使用率超过 85% 且持续 5 分钟时触发告警',
    true, 'disk_usage_percent', '>=', 85.0, 5, 5, 'P2', true, 30, NOW(), NOW()
) ON CONFLICT (name) DO NOTHING;

INSERT INTO ops_alert_rules (
    name, description, enabled, metric_type, operator, threshold,
    window_minutes, sustained_minutes, severity, notify_email, cooldown_minutes,
    created_at, updated_at
) VALUES (
    '磁盘使用率严重过高',
    '当磁盘使用率超过 95% 且持续 3 分钟时触发告警',
    true, 'disk_usage_percent', '>=', 95.0, 5, 3, 'P1', true, 20, NOW(), NOW()
) ON CONFLICT (name) DO NOTHING;
