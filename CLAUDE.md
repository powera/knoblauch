Knoblauch is a simple Slack-like group chat server written in Go with a Postgres backend.

## Structure

    cmd/knoblauch/main.go   — entry point, flags, server wiring
    internal/db/            — pgx/v5 pool setup and all SQL queries
    internal/handler/       — HTTP handlers, SSE broker, session/auth
    internal/model/         — plain data structs (no ORM)
    migrations/             — raw SQL migration files (apply manually)
    templates/              — html/template files (*.html)
    static/css/main.css     — all styles
    static/js/chat.js       — SSE + polling live-update logic

## Tech choices

- **net/http** stdlib router (Go 1.22 method+path syntax, no framework)
- **pgx/v5** for Postgres (connection pool via pgxpool)
- **html/template** server-rendered pages + a thin JS layer for live updates
- **SSE** (Server-Sent Events) for real-time messages, with a polling fallback
- **bcrypt** for password hashing
- **HMAC-SHA256** signed session cookies (no external session store)

## Running locally

1. Install Go 1.22+ and have a local Postgres instance running.
2. Create the database and apply the migration:
   ```
   createdb knoblauch
   psql knoblauch -f migrations/001_initial.sql
   ```
3. Build and run:
   ```
   go build -o bin/knoblauch ./cmd/knoblauch
   ./bin/knoblauch -db "postgres://localhost/knoblauch?sslmode=disable"
   ```
4. Open http://localhost:8080

## Environment variables

| Variable         | Description                              | Default                                          |
|------------------|------------------------------------------|--------------------------------------------------|
| DATABASE_URL     | Postgres connection string               | postgres://localhost/knoblauch?sslmode=disable   |
| SESSION_SECRET   | 64-char hex (32 bytes) for HMAC signing  | generated (ephemeral — sessions lost on restart) |

## Code conventions

- All SQL is in `internal/db/queries.go`. No ORM.
- Handlers return early on error; no panic recovery middleware.
- Use `log/slog` for structured logging.
- Keep business logic out of handlers; handlers should only parse input, call db, and render.
