-- Add subscription-level total quota support.
-- total_limit_usd is configured on the subscription group/plan.
-- total_usage_usd is accumulated on each user subscription and does not reset
-- with daily/weekly/monthly windows.

ALTER TABLE groups
  ADD COLUMN IF NOT EXISTS total_limit_usd DECIMAL(20, 8) DEFAULT NULL;

ALTER TABLE user_subscriptions
  ADD COLUMN IF NOT EXISTS total_usage_usd DECIMAL(20, 10) NOT NULL DEFAULT 0;

COMMENT ON COLUMN groups.total_limit_usd IS 'Subscription total quota limit in USD (NULL = unlimited)';
COMMENT ON COLUMN user_subscriptions.total_usage_usd IS 'Accumulated subscription total usage in USD; not reset by daily/weekly/monthly windows';
