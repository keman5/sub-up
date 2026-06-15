-- Add optional disk / GPU runtime metrics for ops dashboard.
ALTER TABLE ops_system_metrics
    ADD COLUMN IF NOT EXISTS disk_used_gb BIGINT,
    ADD COLUMN IF NOT EXISTS disk_total_gb BIGINT,
    ADD COLUMN IF NOT EXISTS disk_usage_percent DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS gpu_usage_percent DOUBLE PRECISION;

COMMENT ON COLUMN ops_system_metrics.disk_used_gb IS 'Disk used space in GB for runtime root filesystem.';
COMMENT ON COLUMN ops_system_metrics.disk_total_gb IS 'Disk total space in GB for runtime root filesystem.';
COMMENT ON COLUMN ops_system_metrics.disk_usage_percent IS 'Disk usage percentage for runtime root filesystem.';
COMMENT ON COLUMN ops_system_metrics.gpu_usage_percent IS 'GPU usage percentage averaged across visible NVIDIA GPUs.';
