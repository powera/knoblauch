-- 004_user_settings.sql
-- Adds per-user display name, preferred timezone, and avatar URL.

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS display_name TEXT,
  ADD COLUMN IF NOT EXISTS timezone     TEXT NOT NULL DEFAULT 'UTC',
  ADD COLUMN IF NOT EXISTS avatar_url   TEXT;
