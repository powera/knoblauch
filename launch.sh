#!/usr/bin/env bash
export PATH="/usr/local/go/bin:$PATH"
set -e

cd "$(dirname "$0")"

go build -o bin/knoblauch ./cmd/knoblauch

PG_PASSWORD=$(cat keys/postgres.key | tr -d '[:space:]')
export DATABASE_URL="postgresql://postgres.srouvwdghrmwkxnzyzqz:${PG_PASSWORD}@aws-0-us-west-2.pooler.supabase.com:5432/postgres"

if [ ! -f keys/session.key ]; then
  openssl rand -hex 32 > keys/session.key
  echo "Generated new session secret at keys/session.key"
fi
export SESSION_SECRET=$(cat keys/session.key | tr -d '[:space:]')

./bin/knoblauch "$@" &
echo "knoblauch started (PID $!)"
