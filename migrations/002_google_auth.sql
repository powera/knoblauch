-- Migration: add Google OAuth support.
-- psql $DATABASE_URL -f migrations/002_google_auth.sql

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS google_id TEXT UNIQUE,
    ADD COLUMN IF NOT EXISTS email     TEXT,
    ALTER COLUMN password_hash DROP NOT NULL;

CREATE INDEX IF NOT EXISTS users_google_id ON users (google_id);
