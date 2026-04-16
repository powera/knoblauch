package handler_test

import (
	"net/http"
	"testing"
)

const baseURL = "http://localhost:8080"

func get(t *testing.T, path string) *http.Response {
	t.Helper()
	// Don't follow redirects — we want to inspect the redirect itself.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Errorf("GET %s: got %d, want %d", resp.Request.URL.Path, resp.StatusCode, want)
	}
}

func assertRedirectTo(t *testing.T, resp *http.Response, want string) {
	t.Helper()
	loc := resp.Header.Get("Location")
	if loc != want {
		t.Errorf("redirect location: got %q, want %q", loc, want)
	}
}

// TestIndexRedirectsToLogin checks that the home page redirects unauthenticated users to /login.
func TestIndexRedirectsToLogin(t *testing.T) {
	resp := get(t, "/")
	assertStatus(t, resp, http.StatusSeeOther)
	assertRedirectTo(t, resp, "/login")
}

// TestLoginPageOK checks that the login page renders.
func TestLoginPageOK(t *testing.T) {
	resp := get(t, "/login")
	assertStatus(t, resp, http.StatusOK)
}

// TestRegisterPageOK checks that the register page renders.
func TestRegisterPageOK(t *testing.T) {
	resp := get(t, "/register")
	assertStatus(t, resp, http.StatusOK)
}

// TestUnknownPathIs404 checks that an unknown path returns 404.
func TestUnknownPathIs404(t *testing.T) {
	resp := get(t, "/does-not-exist")
	assertStatus(t, resp, http.StatusNotFound)
}

// TestChannelRequiresAuth checks that /channel/* redirects unauthenticated users to /login.
func TestChannelRequiresAuth(t *testing.T) {
	resp := get(t, "/channel/general")
	assertStatus(t, resp, http.StatusSeeOther)
	assertRedirectTo(t, resp, "/login")
}

// TestPollRequiresAuth checks that the poll endpoint rejects unauthenticated requests.
func TestPollRequiresAuth(t *testing.T) {
	resp := get(t, "/channel/general/poll")
	assertStatus(t, resp, http.StatusUnauthorized)
}

// TestSSERequiresAuth checks that the SSE endpoint rejects unauthenticated requests.
func TestSSERequiresAuth(t *testing.T) {
	resp := get(t, "/channel/general/events")
	assertStatus(t, resp, http.StatusUnauthorized)
}
