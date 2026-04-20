package launchagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type fakeLaunchctl struct {
	mu         sync.Mutex
	bootouts   []string
	bootstraps [][2]string
	bootoutErr error
}

func (f *fakeLaunchctl) Bootstrap(_ context.Context, domainTarget, plistPath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bootstraps = append(f.bootstraps, [2]string{domainTarget, plistPath})
	return nil
}

func (f *fakeLaunchctl) Bootout(_ context.Context, serviceTarget string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bootouts = append(f.bootouts, serviceTarget)
	return f.bootoutErr
}

func newTestHandler(t *testing.T) (*Handler, *fakeLaunchctl, string) {
	t.Helper()
	dir := t.TempDir()
	lc := &fakeLaunchctl{}
	h, err := New(
		WithBaseDir(dir),
		WithDomainTarget("gui/501"),
		WithLaunchctl(lc),
	)
	if err != nil {
		t.Fatal(err)
	}
	return h, lc, dir
}

func sampleManifest() *schema.Manifest {
	return &schema.Manifest{
		APIVersion: schema.APIVersionV1,
		Kind:       schema.KindLaunchAgent,
		Metadata:   schema.Metadata{Name: "brew-update"},
		Spec: &schema.LaunchAgentSpec{
			Label:     "com.emkaytec.brew-update",
			Command:   "/opt/homebrew/bin/brew update",
			RunAtLoad: true,
			Schedule: schema.LaunchAgentSchedule{
				Type:            schema.ScheduleTypeInterval,
				IntervalSeconds: 3600,
			},
		},
	}
}

func TestDescribeChange_Create(t *testing.T) {
	h, _, _ := newTestHandler(t)
	m := sampleManifest()

	change, err := h.DescribeChange(context.Background(), m, "brew.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if change.Action != reconcile.ActionCreate {
		t.Fatalf("want ActionCreate, got %q", change.Action)
	}
	if len(change.Drift) != 0 {
		t.Fatalf("create should have no drift, got %+v", change.Drift)
	}
}

func TestDescribeChange_NoOpAfterApply(t *testing.T) {
	h, _, _ := newTestHandler(t)
	m := sampleManifest()

	change, err := h.DescribeChange(context.Background(), m, "brew.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Apply(context.Background(), change, reconcile.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}

	follow, err := h.DescribeChange(context.Background(), m, "brew.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if follow.Action != reconcile.ActionNoOp {
		t.Fatalf("after apply want ActionNoOp, got %q (drift=%+v)", follow.Action, follow.Drift)
	}
}

func TestDescribeChange_UpdateDetectsDrift(t *testing.T) {
	h, _, _ := newTestHandler(t)
	m := sampleManifest()

	change, err := h.DescribeChange(context.Background(), m, "brew.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Apply(context.Background(), change, reconcile.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}

	// Drift the desired spec.
	spec := m.Spec.(*schema.LaunchAgentSpec)
	spec.Command = "/opt/homebrew/bin/brew upgrade"

	follow, err := h.DescribeChange(context.Background(), m, "brew.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if follow.Action != reconcile.ActionUpdate {
		t.Fatalf("want ActionUpdate, got %q", follow.Action)
	}
	if !hasDriftPath(follow.Drift, "spec.command") {
		t.Fatalf("want spec.command drift, got %+v", follow.Drift)
	}
}

func TestApply_WritesFileAndCallsLaunchctl(t *testing.T) {
	h, lc, dir := newTestHandler(t)
	m := sampleManifest()

	change, err := h.DescribeChange(context.Background(), m, "brew.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Apply(context.Background(), change, reconcile.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}

	plistPath := filepath.Join(dir, "com.emkaytec.brew-update.plist")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "com.emkaytec.brew-update") {
		t.Fatalf("plist missing Label: %s", data)
	}

	if len(lc.bootouts) != 1 || lc.bootouts[0] != "gui/501/com.emkaytec.brew-update" {
		t.Fatalf("unexpected bootouts: %+v", lc.bootouts)
	}
	if len(lc.bootstraps) != 1 || lc.bootstraps[0][0] != "gui/501" || lc.bootstraps[0][1] != plistPath {
		t.Fatalf("unexpected bootstraps: %+v", lc.bootstraps)
	}
}

func TestApply_NoOpSkipsWrite(t *testing.T) {
	h, lc, dir := newTestHandler(t)
	change := reconcile.ResourceChange{
		Manifest: sampleManifest(),
		Action:   reconcile.ActionNoOp,
	}

	if err := h.Apply(context.Background(), change, reconcile.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("no-op should not write files, got %d entries", len(entries))
	}
	if len(lc.bootstraps) != 0 || len(lc.bootouts) != 0 {
		t.Fatalf("no-op should not invoke launchctl: %+v / %+v", lc.bootouts, lc.bootstraps)
	}
}

func hasDriftPath(drift []reconcile.DriftField, path string) bool {
	for _, d := range drift {
		if d.Path == path {
			return true
		}
	}
	return false
}
