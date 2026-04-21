package manifest

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestResolveGitHubOwnerPrefersFlag(t *testing.T) {
	p := newPromptSession(strings.NewReader(""), io.Discard)
	configureGitHubRepoFlow(p)

	owner, err := resolveGitHubOwner(context.Background(), p, "  emkaytec  ")
	if err != nil {
		t.Fatalf("resolveGitHubOwner() error = %v", err)
	}
	if owner != "emkaytec" {
		t.Fatalf("owner = %q, want emkaytec", owner)
	}
}

func TestResolveGitHubOwnerDefaultsToAuthenticatedUser(t *testing.T) {
	previous := currentGitHubLogin
	currentGitHubLogin = func(context.Context) string { return "octocat" }
	t.Cleanup(func() { currentGitHubLogin = previous })

	var out strings.Builder
	p := newPromptSession(strings.NewReader("\n"), &out)
	configureGitHubRepoFlow(p)

	owner, err := resolveGitHubOwner(context.Background(), p, "")
	if err != nil {
		t.Fatalf("resolveGitHubOwner() error = %v", err)
	}
	if owner != "octocat" {
		t.Fatalf("owner = %q, want the authenticated login octocat", owner)
	}
	if !strings.Contains(out.String(), "[octocat]") {
		t.Fatalf("expected prompt to advertise octocat as the default, got %q", out.String())
	}
}

func TestScopedManifestNameCombinesOwnerAndApplication(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		owner       string
		application string
		want        string
	}{
		{"owner prefix", "emkaytec", "forge", "emkaytec-forge"},
		{"normalizes owner casing", "EmKayTec", "forge", "emkaytec-forge"},
		{"empty owner falls back to application", "", "forge", "forge"},
		{"already prefixed keeps application", "emkaytec", "emkaytec-forge", "emkaytec-forge"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := scopedManifestName(tc.owner, tc.application); got != tc.want {
				t.Fatalf("scopedManifestName(%q, %q) = %q, want %q", tc.owner, tc.application, got, tc.want)
			}
		})
	}
}

func TestResolveGitHubOwnerAcceptsTypedOverride(t *testing.T) {
	previous := currentGitHubLogin
	currentGitHubLogin = func(context.Context) string { return "octocat" }
	t.Cleanup(func() { currentGitHubLogin = previous })

	p := newPromptSession(strings.NewReader("emkaytec\n"), io.Discard)
	configureGitHubRepoFlow(p)

	owner, err := resolveGitHubOwner(context.Background(), p, "")
	if err != nil {
		t.Fatalf("resolveGitHubOwner() error = %v", err)
	}
	if owner != "emkaytec" {
		t.Fatalf("owner = %q, want the typed override emkaytec", owner)
	}
}
