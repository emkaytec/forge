package manifest

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/aws/accounts"
)

func stubMemberships(t *testing.T, m ghMemberships) {
	t.Helper()
	previous := currentGitHubMemberships
	currentGitHubMemberships = func(context.Context) ghMemberships { return m }
	t.Cleanup(func() { currentGitHubMemberships = previous })
}

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

func TestResolveGitHubOwnerFallsBackToFreeFormWhenNoMemberships(t *testing.T) {
	stubMemberships(t, ghMemberships{})

	p := newPromptSession(strings.NewReader("emkaytec\n"), io.Discard)
	configureGitHubRepoFlow(p)

	owner, err := resolveGitHubOwner(context.Background(), p, "")
	if err != nil {
		t.Fatalf("resolveGitHubOwner() error = %v", err)
	}
	if owner != "emkaytec" {
		t.Fatalf("owner = %q, want free-form entry emkaytec", owner)
	}
}

func TestResolveGitHubOwnerPresentsMembershipSelector(t *testing.T) {
	stubMemberships(t, ghMemberships{
		Login: "octocat",
		Orgs:  []string{"emkaytec", "some-other-org"},
	})

	var out strings.Builder
	p := newPromptSession(strings.NewReader("2\n"), &out)
	configureGitHubRepoFlow(p)

	owner, err := resolveGitHubOwner(context.Background(), p, "")
	if err != nil {
		t.Fatalf("resolveGitHubOwner() error = %v", err)
	}
	if owner != "emkaytec" {
		t.Fatalf("owner = %q, want emkaytec (second option)", owner)
	}

	rendered := out.String()
	for _, want := range []string{
		"octocat (personal)",
		"emkaytec (organization)",
		"some-other-org (organization)",
		"Enter a different owner manually",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("selector output missing %q; got %q", want, rendered)
		}
	}
}

func TestResolveGitHubOwnerSelectorDefaultIsPersonalLogin(t *testing.T) {
	stubMemberships(t, ghMemberships{
		Login: "octocat",
		Orgs:  []string{"emkaytec"},
	})

	// An empty line accepts the default — index 0, the personal login.
	p := newPromptSession(strings.NewReader("\n"), io.Discard)
	configureGitHubRepoFlow(p)

	owner, err := resolveGitHubOwner(context.Background(), p, "")
	if err != nil {
		t.Fatalf("resolveGitHubOwner() error = %v", err)
	}
	if owner != "octocat" {
		t.Fatalf("owner = %q, want the default personal login octocat", owner)
	}
}

func TestResolveGitHubOwnerManualEntryFromSelector(t *testing.T) {
	stubMemberships(t, ghMemberships{
		Login: "octocat",
		Orgs:  []string{"emkaytec"},
	})

	// Selector offers [octocat, emkaytec, manual] — pick option 3 (manual)
	// and then type the owner on the follow-up prompt.
	p := newPromptSession(strings.NewReader("3\nthird-party\n"), io.Discard)
	configureGitHubRepoFlow(p)

	owner, err := resolveGitHubOwner(context.Background(), p, "")
	if err != nil {
		t.Fatalf("resolveGitHubOwner() error = %v", err)
	}
	if owner != "third-party" {
		t.Fatalf("owner = %q, want third-party entered at the manual prompt", owner)
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

func TestManifestNameFromVCSRepo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		vcsRepo string
		want    string
	}{
		{"owner and repo", "emkaytec/forge", "emkaytec-forge"},
		{"normalizes owner and repo", "EmKayTec/ForgeApp", "emkaytec-forge-app"},
		{"preserves hyphenated repo", "emkaytec/test-one", "emkaytec-test-one"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := manifestNameFromVCSRepo(tc.vcsRepo)
			if err != nil {
				t.Fatalf("manifestNameFromVCSRepo() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("manifestNameFromVCSRepo(%q) = %q, want %q", tc.vcsRepo, got, tc.want)
			}
		})
	}
}

func TestWorkspaceNameFromVCSRepoUsesRepoSegment(t *testing.T) {
	t.Parallel()

	got, err := workspaceNameFromVCSRepo("emkaytec/test-one")
	if err != nil {
		t.Fatalf("workspaceNameFromVCSRepo() error = %v", err)
	}
	if got != "test-one" {
		t.Fatalf("workspaceNameFromVCSRepo() = %q, want test-one", got)
	}
}

func TestPrioritizeAWSProfilesMovesEnvironmentMatchesToFront(t *testing.T) {
	t.Parallel()

	profiles := []accounts.Profile{
		{Name: "default", AccountID: "000000000000"},
		{Name: "emkaytec-pre", AccountID: "222222222222"},
		{Name: "emkaytec-dev", AccountID: "111111111111"},
		{Name: "emkaytec-prod", AccountID: "333333333333"},
	}

	ordered, defaultIndex := prioritizeAWSProfiles(profiles, "dev")
	if defaultIndex != 0 {
		t.Fatalf("defaultIndex = %d, want 0", defaultIndex)
	}

	got := []string{
		ordered[0].Name,
		ordered[1].Name,
		ordered[2].Name,
		ordered[3].Name,
	}
	want := []string{"emkaytec-dev", "default", "emkaytec-pre", "emkaytec-prod"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered[%d] = %q, want %q (full order: %#v)", i, got[i], want[i], got)
		}
	}
}

func TestApplicationNameFromVCSRepoUsesRepoSegment(t *testing.T) {
	t.Parallel()

	got, err := applicationNameFromVCSRepo("emkaytec/ForgeApp")
	if err != nil {
		t.Fatalf("applicationNameFromVCSRepo() error = %v", err)
	}
	if got != "forge-app" {
		t.Fatalf("applicationNameFromVCSRepo() = %q, want forge-app", got)
	}
}

func TestDefaultAWSIAMProvisionerTargets(t *testing.T) {
	t.Parallel()

	got, err := defaultAWSIAMProvisionerTargets("emkaytec/forge", "dev")
	if err != nil {
		t.Fatalf("defaultAWSIAMProvisionerTargets() error = %v", err)
	}

	if got["github-actions"] != "emkaytec/forge" {
		t.Fatalf("github target = %q, want emkaytec/forge", got["github-actions"])
	}
	if got["hcp-terraform"] != "emkaytec/*/forge-dev" {
		t.Fatalf("hcp target = %q, want emkaytec/*/forge-dev", got["hcp-terraform"])
	}
}
