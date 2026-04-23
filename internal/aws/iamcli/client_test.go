package iamcli

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	outputs            map[string][]byte
	outputErr          error
	outputWithEnvCalls []outputWithEnvCall
}

type outputWithEnvCall struct {
	env  []string
	args []string
}

func (f *fakeRunner) LookPath(file string) (string, error) {
	return "/usr/bin/" + file, nil
}

func (f *fakeRunner) Output(_ context.Context, _ string, args ...string) ([]byte, error) {
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

func (f *fakeRunner) OutputWithEnv(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	f.outputWithEnvCalls = append(f.outputWithEnvCalls, outputWithEnvCall{
		env:  append([]string(nil), env...),
		args: append([]string(nil), args...),
	})
	return f.Output(ctx, name, args...)
}

func TestGetCallerIdentity(t *testing.T) {
	client := New(WithRunner(&fakeRunner{
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

func TestGetCallerIdentityUsesConfiguredProfile(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string][]byte{
			"sts get-caller-identity": []byte(`{"Account":"123456789012"}`),
		},
	}
	client := New(WithRunner(runner)).ForProfile("prod-admin")

	if _, err := client.GetCallerIdentity(context.Background()); err != nil {
		t.Fatalf("GetCallerIdentity() error = %v", err)
	}
	if len(runner.outputWithEnvCalls) != 1 {
		t.Fatalf("OutputWithEnv calls = %d, want 1", len(runner.outputWithEnvCalls))
	}
	if !containsString(runner.outputWithEnvCalls[0].env, "AWS_PROFILE=prod-admin") {
		t.Fatalf("profile env missing from call: %#v", runner.outputWithEnvCalls[0].env)
	}
}

func TestGetRoleDecodesObjectAssumeRolePolicyDocument(t *testing.T) {
	client := New(WithRunner(&fakeRunner{
		outputs: map[string][]byte{
			"iam get-role": []byte(`{
				"Role": {
					"RoleName": "sample-role",
					"Arn": "arn:aws:iam::123456789012:role/sample-role",
					"AssumeRolePolicyDocument": {
						"Version": "2012-10-17",
						"Statement": [
							{
								"Effect": "Allow",
								"Action": "sts:AssumeRoleWithWebIdentity"
							}
						]
					}
				}
			}`),
		},
	}))

	role, err := client.GetRole(context.Background(), "sample-role")
	if err != nil {
		t.Fatalf("GetRole() error = %v", err)
	}
	for _, want := range []string{`"Version":"2012-10-17"`, `"Action":"sts:AssumeRoleWithWebIdentity"`} {
		if !strings.Contains(role.AssumeRolePolicy, want) {
			t.Fatalf("AssumeRolePolicy = %q, want substring %q", role.AssumeRolePolicy, want)
		}
	}
}

func TestGetRolePolicyDecodesEscapedPolicyDocument(t *testing.T) {
	client := New(WithRunner(&fakeRunner{
		outputs: map[string][]byte{
			"iam get-role-policy": []byte(`{
				"RoleName": "sample-role",
				"PolicyName": "sample-policy",
				"PolicyDocument": "%7B%22Version%22%3A%222012-10-17%22%7D"
			}`),
		},
	}))

	policy, err := client.GetRolePolicy(context.Background(), "sample-role", "sample-policy")
	if err != nil {
		t.Fatalf("GetRolePolicy() error = %v", err)
	}
	if policy != `{"Version":"2012-10-17"}` {
		t.Fatalf("policy = %q", policy)
	}
}

func TestOIDCProviderExistsReturnsFalseOnNoSuchEntity(t *testing.T) {
	client := New(WithRunner(&fakeRunner{
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

func TestIsNoSuchEntityReturnsFalseForNilError(t *testing.T) {
	if IsNoSuchEntity(nil) {
		t.Fatal("IsNoSuchEntity(nil) = true, want false")
	}
}

func TestGetRoleAssumesOrganizationAccountAccessRoleWhenTargetDiffers(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string][]byte{
			"sts get-caller-identity": []byte(`{"Account":"111111111111"}`),
			"sts assume-role": []byte(`{
				"Credentials": {
					"AccessKeyId": "ASIAEXAMPLE",
					"SecretAccessKey": "secret",
					"SessionToken": "session"
				}
			}`),
			"iam get-role": []byte(`{
				"Role": {
					"RoleName": "sample-role",
					"Arn": "arn:aws:iam::222222222222:role/sample-role",
					"AssumeRolePolicyDocument": "%7B%7D"
				}
			}`),
		},
	}
	client := New(WithRunner(runner)).ForAccount("222222222222")

	role, err := client.GetRole(context.Background(), "sample-role")
	if err != nil {
		t.Fatalf("GetRole() error = %v", err)
	}
	if role.ARN != "arn:aws:iam::222222222222:role/sample-role" {
		t.Fatalf("role ARN = %q", role.ARN)
	}
	if len(runner.outputWithEnvCalls) != 1 {
		t.Fatalf("OutputWithEnv calls = %d, want 1", len(runner.outputWithEnvCalls))
	}

	call := runner.outputWithEnvCalls[0]
	if got := strings.Join(call.args, " "); got != "iam get-role --role-name sample-role --output json" {
		t.Fatalf("assumed command args = %q", got)
	}
	for _, want := range []string{
		"AWS_ACCESS_KEY_ID=ASIAEXAMPLE",
		"AWS_SECRET_ACCESS_KEY=secret",
		"AWS_SESSION_TOKEN=session",
	} {
		if !containsString(call.env, want) {
			t.Fatalf("assumed command env missing %q: %#v", want, call.env)
		}
	}
}

func TestGetRoleUsesAmbientCredentialsWhenTargetMatchesCaller(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string][]byte{
			"sts get-caller-identity": []byte(`{"Account":"111111111111"}`),
			"iam get-role": []byte(`{
				"Role": {
					"RoleName": "sample-role",
					"Arn": "arn:aws:iam::111111111111:role/sample-role",
					"AssumeRolePolicyDocument": "%7B%7D"
				}
			}`),
		},
	}
	client := New(WithRunner(runner)).ForAccount("111111111111")

	if _, err := client.GetRole(context.Background(), "sample-role"); err != nil {
		t.Fatalf("GetRole() error = %v", err)
	}
	if len(runner.outputWithEnvCalls) != 0 {
		t.Fatalf("OutputWithEnv calls = %d, want 0", len(runner.outputWithEnvCalls))
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
