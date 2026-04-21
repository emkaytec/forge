package iamcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

type runner interface {
	LookPath(file string) (string, error)
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, &commandError{
			Name:   name,
			Args:   args,
			Err:    err,
			Stderr: strings.TrimSpace(stderr.String()),
		}
	}

	return output, nil
}

type commandError struct {
	Name   string
	Args   []string
	Err    error
	Stderr string
}

func (e *commandError) Error() string {
	command := strings.TrimSpace(strings.Join(append([]string{e.Name}, e.Args...), " "))
	if e.Stderr != "" {
		return fmt.Sprintf("%s failed: %s", command, e.Stderr)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s failed: %v", command, e.Err)
	}
	return command + " failed"
}

func (e *commandError) Unwrap() error {
	return e.Err
}

// Client is a small AWS CLI adapter for the IAM and STS calls Forge needs.
type Client struct {
	runner runner
}

// Option configures a Client.
type Option func(*Client)

// WithRunner injects a custom command runner. Tests use this seam.
func WithRunner(runner runner) Option {
	return func(client *Client) {
		client.runner = runner
	}
}

// New returns a Client backed by the ambient AWS CLI configuration.
func New(opts ...Option) *Client {
	client := &Client{runner: execRunner{}}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// EnsureCLI verifies that the AWS CLI is installed.
func (c *Client) EnsureCLI() error {
	if _, err := c.runner.LookPath("aws"); err != nil {
		return fmt.Errorf("aws cli is required on PATH: %w", err)
	}
	return nil
}

// GetCallerIdentity resolves the current AWS account ID from ambient credentials.
func (c *Client) GetCallerIdentity(ctx context.Context) (string, error) {
	if err := c.EnsureCLI(); err != nil {
		return "", err
	}

	output, err := c.runner.Output(ctx, "aws", "sts", "get-caller-identity", "--output", "json")
	if err != nil {
		return "", err
	}

	var response struct {
		Account string `json:"Account"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return "", fmt.Errorf("decode aws caller identity: %w", err)
	}
	if strings.TrimSpace(response.Account) == "" {
		return "", fmt.Errorf("aws caller identity did not include an account ID")
	}

	return strings.TrimSpace(response.Account), nil
}

// OIDCProviderExists reports whether the provider ARN exists in IAM.
func (c *Client) OIDCProviderExists(ctx context.Context, arn string) (bool, error) {
	if err := c.EnsureCLI(); err != nil {
		return false, err
	}

	_, err := c.runner.Output(ctx, "aws", "iam", "get-open-id-connect-provider", "--open-id-connect-provider-arn", arn, "--output", "json")
	if err == nil {
		return true, nil
	}
	if isNoSuchEntity(err) {
		return false, nil
	}

	return false, err
}

// CreateOIDCProvider creates an OIDC provider and returns its ARN.
func (c *Client) CreateOIDCProvider(ctx context.Context, issuerURL string, audiences []string) (string, error) {
	if err := c.EnsureCLI(); err != nil {
		return "", err
	}

	args := []string{
		"iam",
		"create-open-id-connect-provider",
		"--url",
		issuerURL,
		"--output",
		"json",
	}
	if len(audiences) > 0 {
		args = append(args, "--client-id-list")
		args = append(args, audiences...)
	}

	output, err := c.runner.Output(ctx, "aws", args...)
	if err != nil {
		return "", err
	}

	var response struct {
		ARN string `json:"OpenIDConnectProviderArn"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return "", fmt.Errorf("decode create oidc provider response: %w", err)
	}
	if strings.TrimSpace(response.ARN) == "" {
		return "", fmt.Errorf("create oidc provider response did not include an ARN")
	}

	return response.ARN, nil
}

type Role struct {
	Name             string
	ARN              string
	AssumeRolePolicy string
}

// GetRole fetches one IAM role by name.
func (c *Client) GetRole(ctx context.Context, roleName string) (*Role, error) {
	if err := c.EnsureCLI(); err != nil {
		return nil, err
	}

	output, err := c.runner.Output(ctx, "aws", "iam", "get-role", "--role-name", roleName, "--output", "json")
	if err != nil {
		return nil, err
	}

	var response struct {
		Role struct {
			RoleName                 string `json:"RoleName"`
			ARN                      string `json:"Arn"`
			AssumeRolePolicyDocument string `json:"AssumeRolePolicyDocument"`
		} `json:"Role"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("decode get role response: %w", err)
	}

	policy, err := url.QueryUnescape(response.Role.AssumeRolePolicyDocument)
	if err != nil {
		policy = response.Role.AssumeRolePolicyDocument
	}

	return &Role{
		Name:             response.Role.RoleName,
		ARN:              response.Role.ARN,
		AssumeRolePolicy: policy,
	}, nil
}

// CreateRole creates a role with the given trust policy.
func (c *Client) CreateRole(ctx context.Context, roleName string, assumeRolePolicy string) error {
	if err := c.EnsureCLI(); err != nil {
		return err
	}

	_, err := c.runner.Output(
		ctx,
		"aws",
		"iam",
		"create-role",
		"--role-name",
		roleName,
		"--assume-role-policy-document",
		assumeRolePolicy,
		"--output",
		"json",
	)
	return err
}

// UpdateAssumeRolePolicy replaces the trust policy for an existing role.
func (c *Client) UpdateAssumeRolePolicy(ctx context.Context, roleName string, assumeRolePolicy string) error {
	if err := c.EnsureCLI(); err != nil {
		return err
	}

	_, err := c.runner.Output(
		ctx,
		"aws",
		"iam",
		"update-assume-role-policy",
		"--role-name",
		roleName,
		"--policy-document",
		assumeRolePolicy,
	)
	return err
}

// ListAttachedRolePolicies returns the attached managed policy ARNs for a role.
func (c *Client) ListAttachedRolePolicies(ctx context.Context, roleName string) ([]string, error) {
	if err := c.EnsureCLI(); err != nil {
		return nil, err
	}

	output, err := c.runner.Output(ctx, "aws", "iam", "list-attached-role-policies", "--role-name", roleName, "--output", "json")
	if err != nil {
		return nil, err
	}

	var response struct {
		AttachedPolicies []struct {
			PolicyARN string `json:"PolicyArn"`
		} `json:"AttachedPolicies"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("decode attached role policies response: %w", err)
	}

	policies := make([]string, 0, len(response.AttachedPolicies))
	for _, policy := range response.AttachedPolicies {
		policies = append(policies, policy.PolicyARN)
	}

	return policies, nil
}

// AttachRolePolicy attaches one managed policy ARN to the role.
func (c *Client) AttachRolePolicy(ctx context.Context, roleName string, policyARN string) error {
	if err := c.EnsureCLI(); err != nil {
		return err
	}

	_, err := c.runner.Output(ctx, "aws", "iam", "attach-role-policy", "--role-name", roleName, "--policy-arn", policyARN)
	return err
}

// DetachRolePolicy detaches one managed policy ARN from the role.
func (c *Client) DetachRolePolicy(ctx context.Context, roleName string, policyARN string) error {
	if err := c.EnsureCLI(); err != nil {
		return err
	}

	_, err := c.runner.Output(ctx, "aws", "iam", "detach-role-policy", "--role-name", roleName, "--policy-arn", policyARN)
	return err
}

func isNoSuchEntity(err error) bool {
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		return strings.Contains(commandErr.Stderr, "NoSuchEntity")
	}

	return strings.Contains(err.Error(), "NoSuchEntity")
}

// IsNoSuchEntity reports whether the AWS CLI returned IAM's NoSuchEntity error.
func IsNoSuchEntity(err error) bool {
	return isNoSuchEntity(err)
}
