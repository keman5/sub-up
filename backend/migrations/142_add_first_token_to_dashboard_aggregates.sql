-- Track first-token latency in dashboard pre-aggregation tables.

ALTER TABLE usage_dashboard_hourly
    ADD COLUMN IF NOT EXISTS total_first_token_ms BIGINT NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_hourly
    ADD COLUMN IF NOT EXISTS first_token_requests BIGINT NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_daily
    ADD COLUMN IF NOT EXISTS total_first_token_ms BIGINT NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_daily
    ADD COLUMN IF NOT EXISTS first_token_requests BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN usage_dashboard_hourly.total_first_token_ms IS 'Sum of first_token_ms values for rows with first-token latency in this hour bucket.';
COMMENT ON COLUMN usage_dashboard_hourly.first_token_requests IS 'Number of requests with first_token_ms in this hour bucket.';
COMMENT ON COLUMN usage_dashboard_daily.total_first_token_ms IS 'Sum of first_token_ms values for rows with first-token latency in this day bucket.';
COMMENT ON COLUMN usage_dashboard_daily.first_token_requests IS 'Number of requests with first_token_ms in this day bucket.';
