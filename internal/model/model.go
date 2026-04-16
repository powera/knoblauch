package model

import "time"

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Email        string
	IsSystem     bool
	CreatedAt    time.Time
	DisplayName  string // optional; falls back to Username when empty
	Timezone     string // IANA timezone name, e.g. "America/New_York"
	AvatarURL    string // optional external image URL
}

type Channel struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
}

type Message struct {
	ID          int64
	ChannelID   int64
	UserID      int64
	Username    string // joined from users
	DisplayName string // joined from users; empty means use Username
	Body        string
	BodyHTML    string // pre-rendered safe HTML (populated after DB fetch)
	CreatedAt   time.Time
}
