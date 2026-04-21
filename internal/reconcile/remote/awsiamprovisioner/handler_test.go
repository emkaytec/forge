package awsiamprovisioner

import (
	"context"
	"errors"
	"testing"

	"github.com/emkaytec/forge/internal/aws/iamcli"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type fakeIAMClient struct{}

func (fakeIAMClient) OIDCProviderExists(context.Context, string) (bool, error) { return false, nil }
func (fakeIAMClient) GetRole(context.Context, string) (*iamcli.Role, error) {
	return nil, errors.New("NoSuchEntity")
}
func (fakeIAMClient) CreateRole(context.Context, string, string) error             { return nil }
func (fakeIAMClient) UpdateAssumeRolePolicy(context.Context, string, string) error { return nil }
func (fakeIAMClient) ListAttachedRolePolicies(context.Context, string) ([]string, error) {
	return nil, nil
}
func (fakeIAMClient) AttachRolePolicy(context.Context, string, string) error { return nil }
func (fakeIAMClient) DetachRolePolicy(context.Context, string, string) error { return nil }

func TestDescribeChangeCreatesRoleWhenMissing(t *testing.T) {
	handler := New(WithClientFactory(func() client { return fakeIAMClient{} }))

	change, err := handler.DescribeChange(context.Background(), &schema.Manifest{
		Kind:     schema.KindAWSIAMProvisioner,
		Metadata: schema.Metadata{Name: "gha"},
		Spec: &schema.AWSIAMProvisionerSpec{
			Name:            "gha",
			AccountID:       "123456789012",
			OIDCProvider:    "token.actions.githubusercontent.com",
			OIDCSubject:     "repo:emkaytec/forge:*",
			ManagedPolicies: []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"},
		},
	}, "gha.yaml")
	if err != nil {
		t.Fatalf("DescribeChange() error = %v", err)
	}
	if change.Action != reconcile.ActionCreate {
		t.Fatalf("action = %q, want create", change.Action)
	}
	if change.Note == "" {
		t.Fatal("expected missing-provider note")
	}
}
