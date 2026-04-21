package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// observedTokenServer spins up an httptest server that answers one
// GET /user call and records the Authorization header it received so
// tests can confirm which token landed on the wire.
func observedTokenServer(t *testing.T) (server *httptest.Server, got *string) {
	t.Helper()
	var header string
	got = &header

	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(Account{Login: "observer", Type: "User"})
	})

	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, got
}

func TestNewClientFromEnvPrefersEnvOverGH(t *testing.T) {
	restore := SetLookupGHTokenForTesting(func() string { return "gh-token" })
	t.Cleanup(restore)

	t.Setenv("GITHUB_TOKEN", "env-token")
	t.Setenv("GH_TOKEN", "")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if client.token != "env-token" {
		t.Fatalf("token = %q, want env-token", client.token)
	}

	// Also confirm it actually lands on outgoing requests.
	server, authHeader := observedTokenServer(t)
	client.baseURL = server.URL
	if _, err := client.GetAuthenticatedUser(context.Background()); err != nil {
		t.Fatalf("GetAuthenticatedUser() error = %v", err)
	}
	if got := *authHeader; got != "Bearer env-token" {
		t.Fatalf("Authorization header = %q, want %q", got, "Bearer env-token")
	}
}

func TestNewClientFromEnvFallsBackToGHCLI(t *testing.T) {
	restore := SetLookupGHTokenForTesting(func() string { return "gh-token" })
	t.Cleanup(restore)

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}

	server, authHeader := observedTokenServer(t)
	client.baseURL = server.URL
	if _, err := client.GetAuthenticatedUser(context.Background()); err != nil {
		t.Fatalf("GetAuthenticatedUser() error = %v", err)
	}
	if got := *authHeader; got != "Bearer gh-token" {
		t.Fatalf("Authorization header = %q, want %q", got, "Bearer gh-token")
	}
}

func TestNewClientFromEnvErrorsWhenNothingConfigured(t *testing.T) {
	restore := SetLookupGHTokenForTesting(func() string { return "" })
	t.Cleanup(restore)

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, err := NewClientFromEnv()
	if err == nil {
		t.Fatal("NewClientFromEnv() error = nil, want missing-token error")
	}
	if !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("error = %v, want mention of `gh auth login` in guidance", err)
	}
}

func TestNewClientFromEnvPrefersGHTokenWhenOnlyGHTokenSet(t *testing.T) {
	restore := SetLookupGHTokenForTesting(func() string { return "should-not-be-used" })
	t.Cleanup(restore)

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "gh-token-env")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if client.token != "gh-token-env" {
		t.Fatalf("token = %q, want gh-token-env (GH_TOKEN should beat gh CLI fallback)", client.token)
	}
}
