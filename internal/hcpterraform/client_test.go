package hcpterraform

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestNewClientFromEnvPrefersEnvToken(t *testing.T) {
	t.Setenv("TF_TOKEN_app_terraform_io", "env-token")
	t.Setenv("TFE_TOKEN", "tfe-token")
	t.Setenv("TF_CLI_CONFIG_FILE", filepath.Join(t.TempDir(), "missing.json"))

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if client.token != "env-token" {
		t.Fatalf("token = %q, want env-token", client.token)
	}
}

func TestNewClientFromEnvFallsBackToTerraformCLICredentials(t *testing.T) {
	tempDir := t.TempDir()
	credentialsPath := filepath.Join(tempDir, "credentials.tfrc.json")
	if err := os.WriteFile(credentialsPath, []byte(`{
		"credentials": {
			"app.terraform.io": {
				"token": "cli-token"
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("TF_TOKEN_app_terraform_io", "")
	t.Setenv("TFE_TOKEN", "")
	t.Setenv("TF_CLI_CONFIG_FILE", credentialsPath)

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if client.token != "cli-token" {
		t.Fatalf("token = %q, want cli-token", client.token)
	}
}

func TestNewClientFromEnvErrorsWhenNoTokenIsAvailable(t *testing.T) {
	t.Setenv("TF_TOKEN_app_terraform_io", "")
	t.Setenv("TFE_TOKEN", "")
	t.Setenv("TF_CLI_CONFIG_FILE", filepath.Join(t.TempDir(), "missing.json"))

	_, err := NewClientFromEnv()
	if err == nil {
		t.Fatal("NewClientFromEnv() error = nil, want missing token")
	}
}

func TestIsAlreadyExistsRecognizesTakenErrors(t *testing.T) {
	for _, err := range []*APIError{
		{StatusCode: http.StatusUnprocessableEntity, Method: http.MethodPost, Path: "/organizations/emkaytec/workspaces", Message: "Name has already been taken"},
		{StatusCode: http.StatusUnprocessableEntity, Method: http.MethodPost, Path: "/workspaces/ws-123/vars", Message: "Key has already been taken"},
	} {
		if !IsAlreadyExists(err) {
			t.Fatalf("IsAlreadyExists(%v) = false, want true", err)
		}
	}
}
