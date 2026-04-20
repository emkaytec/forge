// Package launchagent implements the local reconcile handler for
// the LaunchAgent kind. It renders LaunchAgentSpec manifests into
// launchd plist XML, compares them to the live file at
// $HOME/Library/LaunchAgents/<label>.plist, and reloads the agent
// via launchctl on apply.
package launchagent

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

// Handler implements the LaunchAgent local handler contract.
type Handler struct {
	// baseDir is the directory that holds <label>.plist files.
	// Defaults to $HOME/Library/LaunchAgents. Tests override.
	baseDir string
	// domainTarget is the launchctl domain target (e.g. "gui/501").
	// Defaults to "gui/<uid>" for the invoking user. Tests override.
	domainTarget string
	// launchctl is the launchctl adapter. Defaults to execLaunchctl.
	launchctl launchctl
}

// Option configures a Handler. Tests use these to inject fakes.
type Option func(*Handler)

// WithBaseDir overrides the LaunchAgents directory.
func WithBaseDir(dir string) Option { return func(h *Handler) { h.baseDir = dir } }

// WithDomainTarget overrides the launchctl domain target.
func WithDomainTarget(target string) Option {
	return func(h *Handler) { h.domainTarget = target }
}

// WithLaunchctl injects a launchctl adapter.
func WithLaunchctl(l launchctl) Option { return func(h *Handler) { h.launchctl = l } }

// New returns a Handler configured for the current user.
func New(opts ...Option) (*Handler, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("launchagent: resolve home dir: %w", err)
	}

	target, err := defaultDomainTarget()
	if err != nil {
		return nil, err
	}

	h := &Handler{
		baseDir:      filepath.Join(home, "Library", "LaunchAgents"),
		domainTarget: target,
		launchctl:    execLaunchctl{},
	}

	for _, opt := range opts {
		opt(h)
	}

	return h, nil
}

func defaultDomainTarget() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("launchagent: resolve current user: %w", err)
	}
	return "gui/" + u.Uid, nil
}

// Kind reports schema.KindLaunchAgent.
func (h *Handler) Kind() schema.Kind { return schema.KindLaunchAgent }

// DescribeChange reads the live plist (if any), diffs against the
// desired spec, and returns the planned ResourceChange.
func (h *Handler) DescribeChange(_ context.Context, m *schema.Manifest, source string) (reconcile.ResourceChange, error) {
	spec, ok := m.Spec.(*schema.LaunchAgentSpec)
	if !ok {
		return reconcile.ResourceChange{}, fmt.Errorf("launchagent: unexpected spec type %T", m.Spec)
	}

	change := reconcile.ResourceChange{
		Source:   source,
		Manifest: m,
	}

	live, err := h.readLiveSpec(spec.Label)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		change.Action = reconcile.ActionCreate
		return change, nil
	case err != nil:
		return reconcile.ResourceChange{}, fmt.Errorf("launchagent: read live plist: %w", err)
	}

	drift := diffSpecs(spec, live)
	if len(drift) == 0 {
		change.Action = reconcile.ActionNoOp
		return change, nil
	}

	change.Action = reconcile.ActionUpdate
	change.Drift = drift
	return change, nil
}

// Apply renders the plist, writes it atomically, and reloads the
// agent via launchctl. ActionNoOp changes skip the write. DryRun is
// handled by the executor; this method always mutates.
func (h *Handler) Apply(ctx context.Context, change reconcile.ResourceChange, _ reconcile.ApplyOptions) error {
	spec, ok := change.Manifest.Spec.(*schema.LaunchAgentSpec)
	if !ok {
		return fmt.Errorf("launchagent: unexpected spec type %T", change.Manifest.Spec)
	}

	if change.Action == reconcile.ActionNoOp {
		return nil
	}

	data, err := renderPlist(spec)
	if err != nil {
		return err
	}

	path := h.plistPath(spec.Label)
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("launchagent: write plist: %w", err)
	}

	serviceTarget := h.domainTarget + "/" + spec.Label
	if err := h.launchctl.Bootout(ctx, serviceTarget); err != nil {
		return fmt.Errorf("launchagent: bootout %s: %w", serviceTarget, err)
	}

	if err := h.launchctl.Bootstrap(ctx, h.domainTarget, path); err != nil {
		return fmt.Errorf("launchagent: bootstrap %s: %w", h.domainTarget, err)
	}

	return nil
}

func (h *Handler) plistPath(label string) string {
	return filepath.Join(h.baseDir, label+".plist")
}

func (h *Handler) readLiveSpec(label string) (*schema.LaunchAgentSpec, error) {
	data, err := os.ReadFile(h.plistPath(label))
	if err != nil {
		return nil, err
	}

	live, err := parseLivePlist(data)
	if err != nil {
		return nil, err
	}

	return live.toSpec(), nil
}

// writeFileAtomic writes data to path via a temp file + rename so a
// crashed write cannot leave a partial plist in place.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".forge-launchagent-*")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	return os.Rename(tmpName, path)
}
