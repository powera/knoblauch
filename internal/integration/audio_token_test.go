package integration

import (
	"strings"
	"testing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := []byte("test-secret-32-bytes-for-hmac-xx")
	url := "https://example.com/audio/foo.mp3"
	tok, err := SignAudioURL(secret, url)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	got, err := VerifyAudioToken(secret, tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got != url {
		t.Fatalf("url mismatch: got %q want %q", got, url)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	s1 := []byte("secret-one-xxxxxxxxxxxxxxxxxxxxx")
	s2 := []byte("secret-two-xxxxxxxxxxxxxxxxxxxxx")
	tok, err := SignAudioURL(s1, "https://x/y")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := VerifyAudioToken(s2, tok); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestVerifyRejectsTampered(t *testing.T) {
	secret := []byte("test-secret-32-bytes-for-hmac-xx")
	tok, err := SignAudioURL(secret, "https://x/y")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	// Flip one char in the payload segment.
	bad := strings.Replace(tok, tok[:1], "A", 1)
	if bad == tok {
		bad = "B" + tok[1:]
	}
	if _, err := VerifyAudioToken(secret, bad); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	secret := []byte("test-secret-32-bytes-for-hmac-xx")
	if _, err := VerifyAudioToken(secret, "no-dot"); err == nil {
		t.Fatal("expected error for malformed token")
	}
}
