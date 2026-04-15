package model

import "time"

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Email        string
	CreatedAt    time.Time
}

type Channel struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
}

type Message struct {
	ID        int64
	ChannelID int64
	UserID    int64
	Username  string // joined from users
	Body      string
	CreatedAt time.Time
}
