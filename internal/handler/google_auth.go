package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/powera/knoblauch/internal/db"
)

const oauthStateCookie = "oauth_state"

// googleUserInfo is the subset of fields we need from Google's userinfo endpoint.
type googleUserInfo struct {
	Sub           string `json:"sub"`   // stable unique Google user ID
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func NewOAuthConfig(clientID, clientSecret, baseURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  strings.TrimRight(baseURL, "/") + "/auth/google/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

// handleGoogleLogin redirects the user to Google's OAuth consent screen.
func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((10 * time.Minute).Seconds()),
	})
	url := s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// handleGoogleCallback handles the redirect back from Google.
func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state to prevent CSRF.
	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	// Clear the state cookie.
	http.SetCookie(w, &http.Cookie{
		Name:   oauthStateCookie,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	token, err := s.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("oauth token exchange", "err", err)
		http.Error(w, "OAuth token exchange failed", http.StatusInternalServerError)
		return
	}

	info, err := fetchGoogleUserInfo(r.Context(), s.oauthConfig, token)
	if err != nil {
		slog.Error("fetch google user info", "err", err)
		http.Error(w, "failed to fetch user info", http.StatusInternalServerError)
		return
	}

	if !info.EmailVerified {
		http.Error(w, "Google account email is not verified", http.StatusForbidden)
		return
	}

	// Derive a username from the email (part before @), made safe for our schema.
	username := deriveUsername(info.Email)

	u, err := db.UpsertGoogleUser(r.Context(), s.pool, info.Sub, info.Email, username)
	if err != nil {
		slog.Error("upsert google user", "err", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	if err := setSessionCookie(w, s.secret, u.ID, u.Username); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func fetchGoogleUserInfo(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (googleUserInfo, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return googleUserInfo{}, fmt.Errorf("get userinfo: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return googleUserInfo{}, fmt.Errorf("read userinfo body: %w", err)
	}
	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return googleUserInfo{}, fmt.Errorf("unmarshal userinfo: %w", err)
	}
	if info.Sub == "" {
		return googleUserInfo{}, fmt.Errorf("missing sub in userinfo response")
	}
	return info, nil
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// deriveUsername turns an email address into a safe username.
// e.g. "alex@example.com" -> "alex"
func deriveUsername(email string) string {
	local := strings.SplitN(email, "@", 2)[0]
	var sb strings.Builder
	for _, r := range strings.ToLower(local) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			sb.WriteRune(r)
		}
	}
	name := sb.String()
	if len(name) < 2 {
		name = "user"
	}
	if len(name) > 32 {
		name = name[:32]
	}
	return name
}
