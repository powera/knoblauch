package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"

	"github.com/powera/knoblauch/internal/db"
	"github.com/powera/knoblauch/internal/handler"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dsn := flag.String("db", envOrDefault("DATABASE_URL", "postgres://localhost/knoblauch?sslmode=disable"), "Postgres connection string (use sslmode=require for Supabase)")
	secretHex := flag.String("secret", envOrDefault("SESSION_SECRET", ""), "Hex-encoded 32-byte session secret (generated if empty)")
	tmplDir := flag.String("templates", "templates", "Path to templates directory")
	googleClientID := flag.String("google-client-id", envOrDefault("GOOGLE_CLIENT_ID", ""), "Google OAuth client ID")
	googleClientSecret := flag.String("google-client-secret", envOrDefault("GOOGLE_CLIENT_SECRET", ""), "Google OAuth client secret")
	baseURL := flag.String("base-url", envOrDefault("BASE_URL", "http://localhost:8080"), "Public base URL (used for OAuth redirect URI)")
	flag.Parse()

	ctx := context.Background()

	// Session secret
	var secret []byte
	if *secretHex == "" {
		slog.Warn("no SESSION_SECRET set; generating ephemeral secret — sessions won't survive restart")
		var err error
		secret, err = handler.GenerateSecret()
		if err != nil {
			slog.Error("generate secret", "err", err)
			os.Exit(1)
		}
	} else {
		var err error
		secret, err = hex.DecodeString(*secretHex)
		if err != nil || len(secret) != 32 {
			slog.Error("SESSION_SECRET must be a 64-char hex string (32 bytes)")
			os.Exit(1)
		}
	}

	// Database
	pool, err := db.Open(ctx, *dsn)
	if err != nil {
		slog.Error("connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("connected to database")

	// Templates — parse all *.html files in the templates directory
	pattern := filepath.Join(*tmplDir, "*.html")
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"timeAgo": timeAgo,
	}).ParseGlob(pattern)
	if err != nil {
		slog.Error("parse templates", "err", err)
		os.Exit(1)
	}

	// Google OAuth config (optional — omit flags to disable)
	var oauthCfg *oauth2.Config
	if *googleClientID != "" && *googleClientSecret != "" {
		oauthCfg = handler.NewOAuthConfig(*googleClientID, *googleClientSecret, *baseURL)
		slog.Info("Google OAuth enabled")
	} else {
		slog.Warn("GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET not set; Google login disabled")
	}

	// Routes
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	srv := handler.NewServer(pool, tmpl, secret, oauthCfg)
	srv.RegisterRoutes(mux)

	httpServer := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // SSE connections are long-lived
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("listening", "addr", *addr)
	if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("Jan 2")
	}
}
