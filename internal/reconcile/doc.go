// Package reconcile implements the staged planning layer from Forge's
// earlier reconcile command surface, plus the first target executors.
//
// The package follows ADR 0003 (docs/adr/0003-split-reconcile-by-execution-target.md):
// both reconcile targets share the front half — discovery, decode,
// validation, compatibility filtering, and plan construction — and
// swap executors underneath. The public CLI command has been removed,
// but the package split keeps that runtime easy to carve back out into
// anvil later or delete deliberately. Local reconciliation hosted
// workstation-only handlers (LaunchAgent first) because those did not
// have a home in anvil.
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
//   - LaunchAgent: the live plist at
//     $HOME/Library/LaunchAgents/<label>.plist is decoded and compared
//     field-by-field with the desired LaunchAgentSpec. Unknown live
//     fields are ignored (the staged schema is deliberately narrow).
//   - GitHubRepository: visibility, description, default branch,
//     and optional topics.
//   - HCPTerraformWorkspace: execution mode, optional Terraform
//     version, optional project binding, and account_id variable.
//   - AWSIAMProvisioner: OIDC trust policy plus optional exact
//     managed-policy attachments for the role.
package reconcile
