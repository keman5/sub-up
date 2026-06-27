-- Add admin notification settings for subscriptions after they expire.
INSERT INTO settings (key, value, created_at, updated_at)
VALUES
  ('subscription_expired_admin_notify_enabled', 'false', NOW(), NOW()),
  ('subscription_expired_admin_notify_emails', '[]', NOW(), NOW())
ON CONFLICT (key) DO NOTHING;
