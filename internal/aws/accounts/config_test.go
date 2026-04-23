package accounts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfilesMergesConfigAndCredentials(t *testing.T) {
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "config")
	if err := os.WriteFile(configPath, []byte(`
[default]
sso_account_id = 111111111111

[sso-session ignored]
sso_region = us-east-1

[profile prod-admin]
role_arn = arn:aws:iam::222222222222:role/Admin
`), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	credentialsPath := filepath.Join(tempDir, "credentials")
	if err := os.WriteFile(credentialsPath, []byte(`
[sandbox]
aws_access_key_id = test
`), 0o644); err != nil {
		t.Fatalf("WriteFile(credentials) error = %v", err)
	}

	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsPath)

	profiles, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles() error = %v", err)
	}

	if len(profiles) != 3 {
		t.Fatalf("len(profiles) = %d, want 3", len(profiles))
	}

	defaultProfile, ok := FindProfile(profiles, "default")
	if !ok {
		t.Fatal("default profile not found")
	}
	if defaultProfile.AccountID != "111111111111" {
		t.Fatalf("default profile account ID = %q, want 111111111111", defaultProfile.AccountID)
	}

	prodProfile, ok := FindProfile(profiles, "prod-admin")
	if !ok {
		t.Fatal("prod-admin profile not found")
	}
	if prodProfile.AccountID != "222222222222" {
		t.Fatalf("prod-admin account ID = %q, want 222222222222", prodProfile.AccountID)
	}

	sandboxProfile, ok := FindProfile(profiles, "sandbox")
	if !ok {
		t.Fatal("sandbox profile not found")
	}
	if sandboxProfile.AccountID != "" {
		t.Fatalf("sandbox account ID = %q, want empty", sandboxProfile.AccountID)
	}
}

func TestPrioritizeProfilesMovesEnvironmentMatchesToFront(t *testing.T) {
	t.Parallel()

	profiles := []Profile{
		{Name: "default", AccountID: "000000000000"},
		{Name: "emkaytec-pre", AccountID: "222222222222"},
		{Name: "emkaytec-dev", AccountID: "111111111111"},
		{Name: "emkaytec-prod", AccountID: "333333333333"},
	}

	ordered, defaultIndex := PrioritizeProfiles(profiles, "dev")
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

func TestLabelIncludesAccountIDAvailability(t *testing.T) {
	t.Parallel()

	if got := Label(Profile{Name: "prod", AccountID: "123456789012"}); got != "prod (123456789012)" {
		t.Fatalf("Label(with account) = %q", got)
	}
	if got := Label(Profile{Name: "sandbox"}); got != "sandbox (account ID unavailable)" {
		t.Fatalf("Label(without account) = %q", got)
	}
}
