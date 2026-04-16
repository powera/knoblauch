package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/powera/knoblauch/internal/integration"
)

// maxAudioBytes caps how many bytes we will stream from the upstream per request.
// Audio files from Barsukas are typically <1 MB; 25 MB leaves plenty of headroom.
const maxAudioBytes = 25 * 1024 * 1024

// audioClient is the HTTP client used to fetch upstream audio.
// Timeout is on the initial round-trip only (via context); the body copy
// itself is unbounded so that large files can stream.
var audioClient = &http.Client{
	Transport: &http.Transport{
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	},
}

// handleAudioProxy serves GET /audio/{token}. The token is an HMAC-signed
// pointer to an upstream URL (produced by integration.SignAudioURL).
func (s *Server) handleAudioProxy(w http.ResponseWriter, r *http.Request) {
	// Require a valid session — we don't want anonymous internet clients
	// using us as an open proxy for any signed URL that leaks.
	if _, ok := getSession(r, s.secret); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	token := r.PathValue("token")
	upstream, err := integration.VerifyAudioToken(s.secret, token)
	if err != nil {
		http.Error(w, "invalid or expired audio token", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, "bad upstream url", http.StatusBadGateway)
		return
	}
	// Forward Range so the player can seek.
	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}

	resp, err := audioClient.Do(req)
	if err != nil {
		slog.Warn("audio upstream fetch failed", "err", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy a conservative set of headers through.
	for _, h := range []string{"Content-Type", "Content-Length", "Accept-Ranges", "Content-Range", "ETag", "Last-Modified"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "audio/mpeg")
	}
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, max-age=3600")

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, io.LimitReader(resp.Body, maxAudioBytes)); err != nil {
		// Client disconnects are normal for <audio> seeking; don't spam errors.
		slog.Debug("audio proxy copy ended", "err", err)
	}
}
