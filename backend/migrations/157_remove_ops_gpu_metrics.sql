-- Remove GPU runtime metrics from ops dashboard.
ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS gpu_usage_percent;
