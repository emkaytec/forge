package iamcli

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	outputs   map[string][]byte
	outputErr error
}

func (f fakeRunner) LookPath(file string) (string, error) {
	return "/usr/bin/" + file, nil
}

func (f fakeRunner) Output(_ context.Context, _ string, args ...string) ([]byte, error) {
	if f.outputErr != nil {
		return nil, f.outputErr
	}
	if len(args) >= 2 {
		if output, ok := f.outputs[args[0]+" "+args[1]]; ok {
			return output, nil
		}
	}
	return nil, errors.New("unexpected command")
}

func TestGetCallerIdentity(t *testing.T) {
	client := New(WithRunner(fakeRunner{
		outputs: map[string][]byte{
			"sts get-caller-identity": []byte(`{"Account":"123456789012"}`),
		},
	}))

	accountID, err := client.GetCallerIdentity(context.Background())
	if err != nil {
		t.Fatalf("GetCallerIdentity() error = %v", err)
	}
	if accountID != "123456789012" {
		t.Fatalf("accountID = %q, want 123456789012", accountID)
	}
}

func TestOIDCProviderExistsReturnsFalseOnNoSuchEntity(t *testing.T) {
	client := New(WithRunner(fakeRunner{
		outputErr: &commandError{Stderr: "An error occurred (NoSuchEntity) when calling the GetOpenIDConnectProvider operation"},
	}))

	exists, err := client.OIDCProviderExists(context.Background(), "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com")
	if err != nil {
		t.Fatalf("OIDCProviderExists() error = %v", err)
	}
	if exists {
		t.Fatal("expected exists=false")
	}
}
