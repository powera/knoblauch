package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AudioTokenTTL is how long a signed audio token remains valid.
const AudioTokenTTL = 24 * time.Hour

// audioTokenPayload is the JSON body of a signed audio token.
type audioTokenPayload struct {
	URL     string `json:"u"`
	Expires int64  `json:"e"`
}

// audioMACKey derives an HMAC key distinct from the session cookie key by
// mixing in a purpose string — so a leaked audio token can't be replayed as a
// session and vice versa.
func audioMACKey(secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("knoblauch-audio-proxy-v1"))
	return mac.Sum(nil)
}

// SignAudioURL returns an opaque token that lets the bearer fetch url via the
// audio proxy for the next AudioTokenTTL.
func SignAudioURL(secret []byte, url string) (string, error) {
	payload, err := json.Marshal(audioTokenPayload{
		URL:     url,
		Expires: time.Now().Add(AudioTokenTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, audioMACKey(secret))
	mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig, nil
}

// VerifyAudioToken validates a token signed by SignAudioURL and returns the
// upstream URL. Expired or tampered tokens produce an error.
func VerifyAudioToken(secret []byte, token string) (string, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("malformed token")
	}
	enc, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, audioMACKey(secret))
	mac.Write([]byte(enc))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", fmt.Errorf("invalid signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}
	var p audioTokenPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return "", fmt.Errorf("unmarshal payload: %w", err)
	}
	if time.Now().Unix() > p.Expires {
		return "", fmt.Errorf("token expired")
	}
	return p.URL, nil
}
