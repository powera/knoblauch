-- Add is_system flag to distinguish bot/integration users from human users.
-- password_hash is already nullable (from 002_google_auth.sql).

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;

-- Seed the barsukas integration system user.
INSERT INTO users (username, password_hash, is_system)
VALUES ('barsukas', NULL, TRUE)
ON CONFLICT (username) DO UPDATE SET is_system = TRUE;
