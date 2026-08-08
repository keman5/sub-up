-- Add admin notification settings for subscriptions after they expire.
INSERT INTO settings (key, value, updated_at)
VALUES
  ('subscription_expired_admin_notify_enabled', 'false', NOW()),
  ('subscription_expired_admin_notify_emails', '[]', NOW())
ON CONFLICT (key) DO NOTHING;
