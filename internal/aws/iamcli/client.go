package iamcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type runner interface {
	LookPath(file string) (string, error)
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	OutputWithEnv(ctx context.Context, env []string, name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return execRunner{}.output(ctx, nil, name, args...)
}

func (execRunner) OutputWithEnv(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	return execRunner{}.output(ctx, env, name, args...)
}

func (execRunner) output(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
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
	runner             runner
	profileName        string
	targetAccountID    string
	callerAccountID    string
	assumedCredentials map[string]awsCredentials
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
	client := &Client{
		runner:             execRunner{},
		assumedCredentials: map[string]awsCredentials{},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func (c *Client) ForAccount(accountID string) *Client {
	clone := *c
	clone.UseAccount(accountID)
	if clone.assumedCredentials == nil {
		clone.assumedCredentials = map[string]awsCredentials{}
	}
	return &clone
}

func (c *Client) ForProfile(profileName string) *Client {
	clone := *c
	clone.UseProfile(profileName)
	if clone.assumedCredentials == nil {
		clone.assumedCredentials = map[string]awsCredentials{}
	}
	return &clone
}

func (c *Client) UseProfile(profileName string) {
	c.profileName = strings.TrimSpace(profileName)
	c.callerAccountID = ""
}

func (c *Client) UseAccount(accountID string) {
	c.targetAccountID = strings.TrimSpace(accountID)
	if c.assumedCredentials == nil {
		c.assumedCredentials = map[string]awsCredentials{}
	}
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

	output, err := c.baseOutput(ctx, "sts", "get-caller-identity", "--output", "json")
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

func (c *Client) output(ctx context.Context, args ...string) ([]byte, error) {
	if err := c.EnsureCLI(); err != nil {
		return nil, err
	}

	env, err := c.targetAccountEnv(ctx)
	if err != nil {
		return nil, err
	}
	if len(env) == 0 {
		return c.baseOutput(ctx, args...)
	}

	return c.runner.OutputWithEnv(ctx, env, "aws", args...)
}

func (c *Client) baseOutput(ctx context.Context, args ...string) ([]byte, error) {
	env := c.profileEnv()
	if len(env) == 0 {
		return c.runner.Output(ctx, "aws", args...)
	}
	return c.runner.OutputWithEnv(ctx, env, "aws", args...)
}

func (c *Client) profileEnv() []string {
	if strings.TrimSpace(c.profileName) == "" {
		return nil
	}
	return []string{"AWS_PROFILE=" + strings.TrimSpace(c.profileName)}
}

func (c *Client) targetAccountEnv(ctx context.Context) ([]string, error) {
	targetAccountID := strings.TrimSpace(c.targetAccountID)
	if targetAccountID == "" {
		return nil, nil
	}

	callerAccountID, err := c.cachedCallerAccountID(ctx)
	if err != nil {
		return nil, err
	}
	if callerAccountID == targetAccountID {
		return nil, nil
	}

	credentials, ok := c.assumedCredentials[targetAccountID]
	if !ok {
		credentials, err = c.assumeOrganizationAccountAccessRole(ctx, targetAccountID)
		if err != nil {
			return nil, err
		}
		c.assumedCredentials[targetAccountID] = credentials
	}

	return credentials.env(), nil
}

func (c *Client) cachedCallerAccountID(ctx context.Context) (string, error) {
	if strings.TrimSpace(c.callerAccountID) != "" {
		return c.callerAccountID, nil
	}

	accountID, err := c.GetCallerIdentity(ctx)
	if err != nil {
		return "", err
	}
	c.callerAccountID = accountID
	return accountID, nil
}

type awsCredentials struct {
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
}

func (c awsCredentials) env() []string {
	return []string{
		"AWS_ACCESS_KEY_ID=" + c.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY=" + c.SecretAccessKey,
		"AWS_SESSION_TOKEN=" + c.SessionToken,
	}
}

func (c *Client) assumeOrganizationAccountAccessRole(ctx context.Context, accountID string) (awsCredentials, error) {
	output, err := c.baseOutput(
		ctx,
		"sts",
		"assume-role",
		"--role-arn",
		fmt.Sprintf("arn:aws:iam::%s:role/OrganizationAccountAccessRole", accountID),
		"--role-session-name",
		"forge-reconcile",
		"--output",
		"json",
	)
	if err != nil {
		return awsCredentials{}, err
	}

	var response struct {
		Credentials awsCredentials `json:"Credentials"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return awsCredentials{}, fmt.Errorf("decode assume role response: %w", err)
	}

	credentials := response.Credentials
	if strings.TrimSpace(credentials.AccessKeyID) == "" || strings.TrimSpace(credentials.SecretAccessKey) == "" || strings.TrimSpace(credentials.SessionToken) == "" {
		return awsCredentials{}, fmt.Errorf("assume role response did not include complete temporary credentials")
	}

	return credentials, nil
}

// OIDCProviderExists reports whether the provider ARN exists in IAM.
func (c *Client) OIDCProviderExists(ctx context.Context, arn string) (bool, error) {
	_, err := c.output(ctx, "iam", "get-open-id-connect-provider", "--open-id-connect-provider-arn", arn, "--output", "json")
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

	output, err := c.output(ctx, args...)
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
	output, err := c.output(ctx, "iam", "get-role", "--role-name", roleName, "--output", "json")
	if err != nil {
		return nil, err
	}

	var response struct {
		Role struct {
			RoleName                 string          `json:"RoleName"`
			ARN                      string          `json:"Arn"`
			AssumeRolePolicyDocument json.RawMessage `json:"AssumeRolePolicyDocument"`
		} `json:"Role"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("decode get role response: %w", err)
	}

	policy, err := decodeAssumeRolePolicyDocument(response.Role.AssumeRolePolicyDocument)
	if err != nil {
		return nil, fmt.Errorf("decode assume role policy document: %w", err)
	}

	return &Role{
		Name:             response.Role.RoleName,
		ARN:              response.Role.ARN,
		AssumeRolePolicy: policy,
	}, nil
}

func decodeAssumeRolePolicyDocument(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", nil
	}

	var encoded string
	if err := json.Unmarshal(raw, &encoded); err == nil {
		policy, unescapeErr := url.QueryUnescape(encoded)
		if unescapeErr != nil {
			return encoded, nil
		}
		return policy, nil
	}

	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(document)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

// CreateRole creates a role with the given trust policy.
func (c *Client) CreateRole(ctx context.Context, roleName string, assumeRolePolicy string) error {
	_, err := c.output(
		ctx,
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
	_, err := c.output(
		ctx,
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
	output, err := c.output(ctx, "iam", "list-attached-role-policies", "--role-name", roleName, "--output", "json")
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
	_, err := c.output(ctx, "iam", "attach-role-policy", "--role-name", roleName, "--policy-arn", policyARN)
	return err
}

// GetRolePolicy fetches one inline role policy document.
func (c *Client) GetRolePolicy(ctx context.Context, roleName string, policyName string) (string, error) {
	output, err := c.output(ctx, "iam", "get-role-policy", "--role-name", roleName, "--policy-name", policyName, "--output", "json")
	if err != nil {
		return "", err
	}

	var response struct {
		PolicyDocument json.RawMessage `json:"PolicyDocument"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return "", fmt.Errorf("decode get role policy response: %w", err)
	}

	policy, err := decodeAssumeRolePolicyDocument(response.PolicyDocument)
	if err != nil {
		return "", fmt.Errorf("decode role policy document: %w", err)
	}
	return policy, nil
}

// PutRolePolicy creates or replaces one inline policy on a role.
func (c *Client) PutRolePolicy(ctx context.Context, roleName string, policyName string, policyDocument string) error {
	_, err := c.output(
		ctx,
		"iam",
		"put-role-policy",
		"--role-name",
		roleName,
		"--policy-name",
		policyName,
		"--policy-document",
		policyDocument,
	)
	return err
}

// DetachRolePolicy detaches one managed policy ARN from the role.
func (c *Client) DetachRolePolicy(ctx context.Context, roleName string, policyARN string) error {
	_, err := c.output(ctx, "iam", "detach-role-policy", "--role-name", roleName, "--policy-arn", policyARN)
	return err
}

func isNoSuchEntity(err error) bool {
	if err == nil {
		return false
	}

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
