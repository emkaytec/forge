package reconcile_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
)

func TestRenderPlanSaysPlanNotBuiltWhenLoadErrorsExist(t *testing.T) {
	t.Parallel()

	plan := &reconcile.Plan{
		Target: reconcile.TargetRemote,
		LoadErrors: []reconcile.LoadError{
			{Source: "repo.yaml", Err: errors.New("missing GitHub token")},
		},
	}

	var buf bytes.Buffer
	reconcile.RenderPlan(&buf, plan)

	out := buf.String()
	if !strings.Contains(out, "plan not built") {
		t.Fatalf("expected 'plan not built' messaging, got %q", out)
	}
	if strings.Contains(out, "no changes planned") {
		t.Fatalf("did not expect the reassuring 'no changes planned' line when errors exist, got %q", out)
	}
	if !strings.Contains(out, "missing GitHub token") {
		t.Fatalf("expected the error message to be rendered, got %q", out)
	}
}

func TestRenderPlanStillSaysNoChangesWhenNoErrors(t *testing.T) {
	t.Parallel()

	plan := &reconcile.Plan{Target: reconcile.TargetRemote}

	var buf bytes.Buffer
	reconcile.RenderPlan(&buf, plan)

	if !strings.Contains(buf.String(), "no changes planned") {
		t.Fatalf("expected 'no changes planned' when no errors, got %q", buf.String())
	}
}
