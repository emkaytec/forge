package iamcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func isNoSuchEntity(err error) bool {
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		return strings.Contains(commandErr.Stderr, "NoSuchEntity")
	}

	return strings.Contains(err.Error(), "NoSuchEntity")
}
