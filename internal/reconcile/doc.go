// Package reconcile implements the shared planning layer behind
// forge reconcile local and forge reconcile remote, plus the first
// target executors.
//
// The package follows ADR 0003 (docs/adr/0003-split-reconcile-by-execution-target.md):
// both reconcile targets share the front half — discovery, decode,
// validation, compatibility filtering, and plan construction — and
// swap executors underneath. Forge owns the operator CLI and target
// selection. Remote reconciliation is a shell around the cloud
// runtime boundary that will later live in anvil; local
// reconciliation hosts workstation-only handlers (launch-agent first)
// because those do not have a home in anvil.
//
// # Idempotency guarantees
//
// No state file. Live API state and the local filesystem are the
// source of truth. Re-applying the same manifest against unchanged
// live state produces an ActionNoOp.
//
// # Compatibility filtering
//
// Each Kind is compatible with a fixed set of Targets (see
// compat.go). During planning, manifests whose Kind is not
// compatible with the requested Target land in Plan.Skipped with a
// SkipReason. Apply honours --strict by failing when Plan.Skipped is
// non-empty. Mixed manifest trees are allowed; target filtering
// decides what executes.
//
// # Drift
//
// A ResourceChange carries Drift entries when the live state
// disagrees with the desired spec. Each target handler decides what
// drift means for its Kind:
//
//   - launch-agent: the live plist at
//     $HOME/Library/LaunchAgents/<label>.plist is decoded and compared
//     field-by-field with the desired LaunchAgentSpec. Unknown live
//     fields are ignored (the staged schema is deliberately narrow).
//   - remote kinds: stubbed in MK-9. Handlers report ActionNoOp and
//     ErrNotImplemented on Apply. Per-kind subpackages under
//     remote/ are the seam anvil will fill in later.
package reconcile
