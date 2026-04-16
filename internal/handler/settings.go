package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/powera/knoblauch/internal/db"
)

// commonTimezones is a curated list of IANA timezone names for the settings dropdown,
// ordered east to west (Pacific/Asia → Europe → Americas).
var commonTimezones = []string{
	"Pacific/Auckland",
	"Australia/Sydney",
	"Australia/Melbourne",
	"Asia/Seoul",
	"Asia/Tokyo",
	"Asia/Shanghai",
	"Asia/Singapore",
	"Asia/Bangkok",
	"Asia/Dhaka",
	"Asia/Kolkata",
	"Asia/Dubai",
	"Europe/Istanbul",
	"Europe/Moscow",
	"Europe/Athens",
	"Europe/Helsinki",
	"Europe/Warsaw",
	"Europe/Stockholm",
	"Europe/Amsterdam",
	"Europe/Berlin",
	"Europe/Rome",
	"Europe/Madrid",
	"Europe/Paris",
	"Europe/London",
	"UTC",
	"America/Argentina/Buenos_Aires",
	"America/Sao_Paulo",
	"America/New_York",
	"America/Toronto",
	"America/Chicago",
	"America/Mexico_City",
	"America/Denver",
	"America/Los_Angeles",
	"America/Vancouver",
	"America/Anchorage",
	"Pacific/Honolulu",
}

// TimezoneOption pairs an IANA name with a human-readable label including the GMT offset.
type TimezoneOption struct {
	Value string
	Label string
}

// buildTimezoneOptions returns TimezoneOption entries with current UTC offsets.
func buildTimezoneOptions() []TimezoneOption {
	now := time.Now()
	opts := make([]TimezoneOption, 0, len(commonTimezones))
	for _, name := range commonTimezones {
		loc, err := time.LoadLocation(name)
		if err != nil {
			opts = append(opts, TimezoneOption{Value: name, Label: name})
			continue
		}
		_, offset := now.In(loc).Zone()
		hours := offset / 3600
		mins := (offset % 3600) / 60
		if mins < 0 {
			mins = -mins
		}
		var label string
		if mins != 0 {
			label = fmt.Sprintf("GMT%+03d:%02d — %s", hours, mins, name)
		} else {
			label = fmt.Sprintf("GMT%+03d:00 — %s", hours, name)
		}
		opts = append(opts, TimezoneOption{Value: name, Label: label})
	}
	return opts
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
		"Timezones": buildTimezoneOptions(),
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
			"Timezones": buildTimezoneOptions(),
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
		"Timezones": buildTimezoneOptions(),
		"Success":   true,
		"Error":     "",
	})
}
