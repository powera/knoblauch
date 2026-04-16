package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/powera/knoblauch/internal/markup"
	"github.com/powera/knoblauch/internal/model"
)

// --- Users ---

func CreateUser(ctx context.Context, pool *pgxpool.Pool, username, passwordHash string) (model.User, error) {
	var u model.User
	err := pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, username, password_hash, created_at`,
		username, passwordHash,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func GetUserByUsername(ctx context.Context, pool *pgxpool.Pool, username string) (model.User, error) {
	var u model.User
	err := pool.QueryRow(ctx,
		`SELECT id, username, COALESCE(password_hash, ''), is_system, created_at FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsSystem, &u.CreatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

// GetSystemUserByUsername fetches a system/bot user (is_system=TRUE) by username.
func GetSystemUserByUsername(ctx context.Context, pool *pgxpool.Pool, username string) (model.User, error) {
	var u model.User
	err := pool.QueryRow(ctx,
		`SELECT id, username, is_system, created_at FROM users WHERE username = $1 AND is_system = TRUE`,
		username,
	).Scan(&u.ID, &u.Username, &u.IsSystem, &u.CreatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("get system user: %w", err)
	}
	return u, nil
}

// GetUserByGoogleID looks up a user by their Google subject ID.
func GetUserByGoogleID(ctx context.Context, pool *pgxpool.Pool, googleID string) (model.User, error) {
	var u model.User
	err := pool.QueryRow(ctx,
		`SELECT id, username, COALESCE(email, ''), created_at FROM users WHERE google_id = $1`,
		googleID,
	).Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("get user by google id: %w", err)
	}
	return u, nil
}

// UpsertGoogleUser creates a new user for the given Google account, or returns the existing one.
// username is only used on first insert; subsequent logins update the email.
func UpsertGoogleUser(ctx context.Context, pool *pgxpool.Pool, googleID, email, username string) (model.User, error) {
	var u model.User
	err := pool.QueryRow(ctx,
		`INSERT INTO users (google_id, email, username)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (google_id) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id, username, COALESCE(email, ''), created_at`,
		googleID, email, username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("upsert google user: %w", err)
	}
	return u, nil
}

// GetUserByID fetches a user by their primary key, including settings fields.
func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id int64) (model.User, error) {
	var u model.User
	err := pool.QueryRow(ctx,
		`SELECT id, username, COALESCE(email, ''), is_system, created_at,
		        COALESCE(display_name, ''), COALESCE(timezone, 'UTC'), COALESCE(avatar_url, '')
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.IsSystem, &u.CreatedAt,
		&u.DisplayName, &u.Timezone, &u.AvatarURL)
	if err != nil {
		return model.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// UpdateUserSettings saves the mutable profile fields for a user.
func UpdateUserSettings(ctx context.Context, pool *pgxpool.Pool, userID int64, displayName, timezone, avatarURL string) error {
	_, err := pool.Exec(ctx,
		`UPDATE users SET display_name = $1, timezone = $2, avatar_url = $3 WHERE id = $4`,
		displayName, timezone, avatarURL, userID,
	)
	if err != nil {
		return fmt.Errorf("update user settings: %w", err)
	}
	return nil
}

// --- Channels ---

func ListChannels(ctx context.Context, pool *pgxpool.Pool) ([]model.Channel, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, name, description, created_at FROM channels ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var channels []model.Channel
	for rows.Next() {
		var c model.Channel
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

func GetChannelByName(ctx context.Context, pool *pgxpool.Pool, name string) (model.Channel, error) {
	var c model.Channel
	err := pool.QueryRow(ctx,
		`SELECT id, name, description, created_at FROM channels WHERE name = $1`,
		name,
	).Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt)
	if err != nil {
		return model.Channel{}, fmt.Errorf("get channel: %w", err)
	}
	return c, nil
}

func CreateChannel(ctx context.Context, pool *pgxpool.Pool, name, description string) (model.Channel, error) {
	var c model.Channel
	err := pool.QueryRow(ctx,
		`INSERT INTO channels (name, description)
		 VALUES ($1, $2)
		 RETURNING id, name, description, created_at`,
		name, description,
	).Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt)
	if err != nil {
		return model.Channel{}, fmt.Errorf("create channel: %w", err)
	}
	return c, nil
}

// --- Messages ---

// RecentMessages returns the most recent limit messages in a channel, oldest-first.
func RecentMessages(ctx context.Context, pool *pgxpool.Pool, channelID int64, limit int) ([]model.Message, error) {
	rows, err := pool.Query(ctx,
		`SELECT m.id, m.channel_id, m.user_id, u.username, m.body, m.created_at
		 FROM messages m
		 JOIN users u ON u.id = m.user_id
		 WHERE m.channel_id = $1
		 ORDER BY m.created_at DESC
		 LIMIT $2`,
		channelID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent messages: %w", err)
	}
	defer rows.Close()

	var msgs []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.UserID, &m.Username, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.BodyHTML = string(markup.RenderBody(m.Body))
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// reverse so oldest is first
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// MessagesSinceID returns messages in a channel with id > afterID, oldest-first.
func MessagesSinceID(ctx context.Context, pool *pgxpool.Pool, channelID, afterID int64) ([]model.Message, error) {
	rows, err := pool.Query(ctx,
		`SELECT m.id, m.channel_id, m.user_id, u.username, m.body, m.created_at
		 FROM messages m
		 JOIN users u ON u.id = m.user_id
		 WHERE m.channel_id = $1 AND m.id > $2
		 ORDER BY m.created_at ASC`,
		channelID, afterID,
	)
	if err != nil {
		return nil, fmt.Errorf("messages since id: %w", err)
	}
	defer rows.Close()

	var msgs []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.UserID, &m.Username, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.BodyHTML = string(markup.RenderBody(m.Body))
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func PostMessage(ctx context.Context, pool *pgxpool.Pool, channelID, userID int64, body string) (model.Message, error) {
	var m model.Message
	err := pool.QueryRow(ctx,
		`WITH ins AS (
		   INSERT INTO messages (channel_id, user_id, body)
		   VALUES ($1, $2, $3)
		   RETURNING id, channel_id, user_id, body, created_at
		 )
		 SELECT ins.id, ins.channel_id, ins.user_id, u.username, ins.body, ins.created_at
		 FROM ins JOIN users u ON u.id = ins.user_id`,
		channelID, userID, body,
	).Scan(&m.ID, &m.ChannelID, &m.UserID, &m.Username, &m.Body, &m.CreatedAt)
	if err != nil {
		return model.Message{}, fmt.Errorf("post message: %w", err)
	}
	m.BodyHTML = string(markup.RenderBody(m.Body))
	return m, nil
}
