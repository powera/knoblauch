package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const sessionCookieName = "knoblauch_session"

type sessionData struct {
	UserID   int64  `json:"uid"`
	Username string `json:"un"`
	Expires  int64  `json:"exp"`
}

// signSession encodes a signed session cookie value.
func signSession(secret []byte, d sessionData) (string, error) {
	payload, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig, nil
}

// verifySession parses and validates a signed session cookie value.
func verifySession(secret []byte, value string) (sessionData, error) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return sessionData{}, fmt.Errorf("malformed session")
	}
	enc, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(enc))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return sessionData{}, fmt.Errorf("invalid session signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return sessionData{}, fmt.Errorf("decode session: %w", err)
	}
	var d sessionData
	if err := json.Unmarshal(payload, &d); err != nil {
		return sessionData{}, fmt.Errorf("unmarshal session: %w", err)
	}
	if time.Now().Unix() > d.Expires {
		return sessionData{}, fmt.Errorf("session expired")
	}
	return d, nil
}

func setSessionCookie(w http.ResponseWriter, secret []byte, userID int64, username string) error {
	d := sessionData{
		UserID:   userID,
		Username: username,
		Expires:  time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	value, err := signSession(secret, d)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 60 * 60,
	})
	return nil
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

func getSession(r *http.Request, secret []byte) (sessionData, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return sessionData{}, false
	}
	d, err := verifySession(secret, c.Value)
	if err != nil {
		return sessionData{}, false
	}
	return d, true
}

// GenerateSecret produces a random 32-byte secret suitable for HMAC signing.
func GenerateSecret() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
