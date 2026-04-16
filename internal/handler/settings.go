package handler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/powera/knoblauch/internal/db"
)

// commonTimezones is a curated list of IANA timezone names for the settings dropdown.
var commonTimezones = []string{
	"UTC",
	"America/New_York",
	"America/Chicago",
	"America/Denver",
	"America/Los_Angeles",
	"America/Anchorage",
	"Pacific/Honolulu",
	"America/Toronto",
	"America/Vancouver",
	"America/Mexico_City",
	"America/Sao_Paulo",
	"America/Argentina/Buenos_Aires",
	"Europe/London",
	"Europe/Paris",
	"Europe/Berlin",
	"Europe/Rome",
	"Europe/Madrid",
	"Europe/Amsterdam",
	"Europe/Stockholm",
	"Europe/Warsaw",
	"Europe/Helsinki",
	"Europe/Athens",
	"Europe/Moscow",
	"Europe/Istanbul",
	"Asia/Dubai",
	"Asia/Kolkata",
	"Asia/Dhaka",
	"Asia/Bangkok",
	"Asia/Singapore",
	"Asia/Shanghai",
	"Asia/Tokyo",
	"Asia/Seoul",
	"Australia/Sydney",
	"Australia/Melbourne",
	"Pacific/Auckland",
}

func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	sess, ok := getSession(r, s.secret)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	u, err := db.GetUserByID(r.Context(), s.pool, sess.UserID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	channels, err := db.ListChannels(r.Context(), s.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.render(w, "settings.html", map[string]any{
		"LoggedIn":  true,
		"Username":  sess.Username,
		"User":      u,
		"Channels":  channels,
		"Timezones": commonTimezones,
		"Success":   false,
		"Error":     "",
	})
}

func (s *Server) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	sess, ok := getSession(r, s.secret)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	timezone := strings.TrimSpace(r.FormValue("timezone"))
	avatarURL := strings.TrimSpace(r.FormValue("avatar_url"))

	renderErr := func(msg string) {
		u, _ := db.GetUserByID(r.Context(), s.pool, sess.UserID)
		// Reflect the submitted values back so the user doesn't lose their edits.
		u.DisplayName = displayName
		u.Timezone = timezone
		u.AvatarURL = avatarURL
		channels, _ := db.ListChannels(r.Context(), s.pool)
		s.render(w, "settings.html", map[string]any{
			"LoggedIn":  true,
			"Username":  sess.Username,
			"User":      u,
			"Channels":  channels,
			"Timezones": commonTimezones,
			"Success":   false,
			"Error":     msg,
		})
	}

	if len(displayName) > 64 {
		renderErr("Display name must be 64 characters or fewer.")
		return
	}

	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		renderErr("Unknown timezone. Please select one from the list.")
		return
	}

	if avatarURL != "" {
		parsed, err := url.ParseRequestURI(avatarURL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			renderErr("Avatar URL must be a valid http or https URL.")
			return
		}
	}

	if err := db.UpdateUserSettings(r.Context(), s.pool, sess.UserID, displayName, timezone, avatarURL); err != nil {
		renderErr("Failed to save settings. Please try again.")
		return
	}

	u, err := db.GetUserByID(r.Context(), s.pool, sess.UserID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	channels, err := db.ListChannels(r.Context(), s.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.render(w, "settings.html", map[string]any{
		"LoggedIn":  true,
		"Username":  sess.Username,
		"User":      u,
		"Channels":  channels,
		"Timezones": commonTimezones,
		"Success":   true,
		"Error":     "",
	})
}
