package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/powera/knoblauch/internal/db"
	"github.com/powera/knoblauch/internal/integration"
	"github.com/powera/knoblauch/internal/model"
)

// Server holds shared state for all handlers.
type Server struct {
	pool        *pgxpool.Pool
	templates   map[string]*template.Template
	secret      []byte
	oauthConfig *oauth2.Config

	// SSE broker
	mu          sync.Mutex
	subscribers map[int64][]chan model.Message // channelID -> list of subscriber chans

	// Integration registry for @mention bot dispatch
	integrations   *integration.Registry
	systemUserIDs  map[string]int64 // integration name -> system user ID
}

func NewServer(pool *pgxpool.Pool, tmpl map[string]*template.Template, secret []byte, oauthCfg *oauth2.Config) *Server {
	return &Server{
		pool:          pool,
		templates:     tmpl,
		secret:        secret,
		oauthConfig:   oauthCfg,
		subscribers:   make(map[int64][]chan model.Message),
		integrations:  integration.NewRegistry(),
		systemUserIDs: make(map[string]int64),
	}
}

// RegisterIntegration adds an integration to the server and looks up its system user ID.
// Call this before the server starts accepting requests.
func (s *Server) RegisterIntegration(ctx context.Context, intg integration.Integration) {
	s.integrations.Register(intg)
	u, err := db.GetSystemUserByUsername(ctx, s.pool, intg.Name())
	if err != nil {
		slog.Warn("system user not found for integration; @mention responses disabled",
			"integration", intg.Name(), "err", err)
		return
	}
	s.systemUserIDs[intg.Name()] = u.ID
	slog.Info("integration registered", "name", intg.Name(), "system_user_id", u.ID)
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /login", s.handleLoginPage)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("GET /register", s.handleRegisterPage)
	mux.HandleFunc("POST /register", s.handleRegister)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /channel/{name}", s.handleChannelPage)
	mux.HandleFunc("POST /channel/{name}/post", s.handlePost)
	mux.HandleFunc("GET /channel/{name}/events", s.handleSSE)
	mux.HandleFunc("GET /channel/{name}/poll", s.handlePoll)
	mux.HandleFunc("GET /channels/new", s.handleNewChannelPage)
	mux.HandleFunc("POST /channels/new", s.handleNewChannel)

	if s.oauthConfig != nil {
		mux.HandleFunc("GET /auth/google", s.handleGoogleLogin)
		mux.HandleFunc("GET /auth/google/callback", s.handleGoogleCallback)
	}
}

// --- SSE broker ---

func (s *Server) subscribe(channelID int64) chan model.Message {
	ch := make(chan model.Message, 16)
	s.mu.Lock()
	s.subscribers[channelID] = append(s.subscribers[channelID], ch)
	s.mu.Unlock()
	return ch
}

func (s *Server) unsubscribe(channelID int64, ch chan model.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subs := s.subscribers[channelID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[channelID] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

func (s *Server) broadcast(channelID int64, msg model.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.subscribers[channelID] {
		select {
		case ch <- msg:
		default: // slow subscriber; drop rather than block
		}
	}
}

// --- Handlers ---

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	sess, ok := getSession(r, s.secret)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	channels, err := db.ListChannels(r.Context(), s.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.render(w, "index.html", map[string]any{
		"LoggedIn": ok,
		"Username": sess.Username,
		"Channels": channels,
	})
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "login.html", map[string]any{
		"Error":         "",
		"GoogleEnabled": s.oauthConfig != nil,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	u, err := db.GetUserByUsername(r.Context(), s.pool, username)
	if err != nil || !checkPassword(password, u.PasswordHash) {
		s.render(w, "login.html", map[string]any{
			"Error":         "Invalid username or password.",
			"GoogleEnabled": s.oauthConfig != nil,
		})
		return
	}
	if err := setSessionCookie(w, s.secret, u.ID, u.Username); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "register.html", map[string]any{"Error": ""})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if len(username) < 2 || len(username) > 32 {
		s.render(w, "register.html", map[string]any{"Error": "Username must be 2–32 characters."})
		return
	}
	if len(password) < 8 {
		s.render(w, "register.html", map[string]any{"Error": "Password must be at least 8 characters."})
		return
	}

	hash, err := hashPassword(password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	u, err := db.CreateUser(r.Context(), s.pool, username, hash)
	if err != nil {
		s.render(w, "register.html", map[string]any{"Error": "Username already taken."})
		return
	}
	if err := setSessionCookie(w, s.secret, u.ID, u.Username); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleChannelPage(w http.ResponseWriter, r *http.Request) {
	sess, ok := getSession(r, s.secret)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	channelName := r.PathValue("name")
	ch, err := db.GetChannelByName(r.Context(), s.pool, channelName)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	msgs, err := db.RecentMessages(r.Context(), s.pool, ch.ID, 50)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	channels, err := db.ListChannels(r.Context(), s.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	lastID := int64(0)
	if len(msgs) > 0 {
		lastID = msgs[len(msgs)-1].ID
	}

	s.render(w, "channel.html", map[string]any{
		"LoggedIn":     true,
		"Username":     sess.Username,
		"Channel":      ch,
		"Messages":     msgs,
		"LastID":       lastID,
		"Channels":     channels,
		"Integrations": s.integrations.Names(),
	})
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	sess, ok := getSession(r, s.secret)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	channelName := r.PathValue("name")
	ch, err := db.GetChannelByName(r.Context(), s.pool, channelName)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Error(w, "empty message", http.StatusBadRequest)
		return
	}
	if len(body) > 4000 {
		http.Error(w, "message too long", http.StatusBadRequest)
		return
	}

	msg, err := db.PostMessage(r.Context(), s.pool, ch.ID, sess.UserID, body)
	if err != nil {
		slog.Error("post message", "err", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.broadcast(ch.ID, msg)

	// Dispatch @mention integrations after saving the user message.
	s.dispatchIntegration(r.Context(), ch.ID, body)

	// HTMX or plain form: if HTMX, return rendered message fragment; else redirect.
	if r.Header.Get("HX-Request") == "true" {
		s.renderFragment(w, "message_row.html", msg)
		return
	}
	http.Redirect(w, r, "/channel/"+channelName, http.StatusSeeOther)
}

// dispatchIntegration checks whether body is an @mention for a registered integration.
// If so, the bot response is saved and broadcast as a message from the system user.
// Runs synchronously so that errors are logged; callers do not need to wait on it.
func (s *Server) dispatchIntegration(ctx context.Context, channelID int64, body string) {
	name, _, ok := integration.ParseMention(body)
	if !ok {
		return
	}
	sysUserID, known := s.systemUserIDs[name]
	if !known {
		return // integration not registered or system user missing
	}
	responseBody, handled := s.integrations.Dispatch(ctx, body)
	if !handled {
		return
	}
	botMsg, err := db.PostMessage(ctx, s.pool, channelID, sysUserID, responseBody)
	if err != nil {
		slog.Error("post integration response", "integration", name, "err", err)
		return
	}
	s.broadcast(channelID, botMsg)
}

// handleSSE streams new messages via Server-Sent Events.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	_, ok := getSession(r, s.secret)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	channelName := r.PathValue("name")
	ch, err := db.GetChannelByName(r.Context(), s.pool, channelName)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := s.subscribe(ch.ID)
	defer s.unsubscribe(ch.ID, sub)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send a keep-alive comment every 15 seconds.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-sub:
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

// handlePoll supports clients that prefer simple polling over SSE.
// GET /channel/{name}/poll?after=<lastID> returns JSON array of new messages.
func (s *Server) handlePoll(w http.ResponseWriter, r *http.Request) {
	_, ok := getSession(r, s.secret)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	channelName := r.PathValue("name")
	ch, err := db.GetChannelByName(r.Context(), s.pool, channelName)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	afterID, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	msgs, err := db.MessagesSinceID(r.Context(), s.pool, ch.ID, afterID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

func (s *Server) handleNewChannelPage(w http.ResponseWriter, r *http.Request) {
	_, ok := getSession(r, s.secret)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	channels, err := db.ListChannels(r.Context(), s.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.render(w, "new_channel.html", map[string]any{
		"LoggedIn": true,
		"Channels": channels,
		"Error":    "",
	})
}

func (s *Server) handleNewChannel(w http.ResponseWriter, r *http.Request) {
	_, ok := getSession(r, s.secret)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if len(name) < 1 || len(name) > 64 {
		channels, _ := db.ListChannels(r.Context(), s.pool)
		s.render(w, "new_channel.html", map[string]any{
			"LoggedIn": true,
			"Channels": channels,
			"Error":    "Channel name must be 1–64 characters.",
		})
		return
	}
	// Allow only lowercase letters, numbers, hyphens, underscores.
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			channels, _ := db.ListChannels(r.Context(), s.pool)
			s.render(w, "new_channel.html", map[string]any{
				"LoggedIn": true,
				"Channels": channels,
				"Error":    "Channel name may only contain lowercase letters, numbers, hyphens, and underscores.",
			})
			return
		}
	}

	_, err := db.CreateChannel(r.Context(), s.pool, name, description)
	if err != nil {
		channels, _ := db.ListChannels(r.Context(), s.pool)
		s.render(w, "new_channel.html", map[string]any{
			"LoggedIn": true,
			"Channels": channels,
			"Error":    "A channel with that name already exists.",
		})
		return
	}
	http.Redirect(w, r, "/channel/"+name, http.StatusSeeOther)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	tmpl, ok := s.templates[name]
	if !ok {
		slog.Error("template not found", "name", name)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("render template", "name", name, "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func (s *Server) renderFragment(w http.ResponseWriter, name string, data any) {
	tmpl, ok := s.templates[name]
	if !ok {
		slog.Error("template not found", "name", name)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("render fragment", "name", name, "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
