# reconcile

Shared planning layer and target executors behind `forge reconcile local` and `forge reconcile remote`.

Package docs, including idempotency guarantees, compatibility filtering, and drift semantics per resource type, live in [`doc.go`](doc.go). Architectural rationale for the target split lives in [ADR 0003](../../docs/adr/0003-split-reconcile-by-execution-target.md).

- **Shared front half** (`compat.go`, `load.go`, `executor.go`, `plan.go`, `report.go`) — discovery, decode, validation, compatibility filtering, plan construction, and output rendering.
- **Remote** (`remote/`) — dispatcher plus one subpackage per remote-capable kind. The current embedded engine manages `GitHubRepository`, `HCPTerraformWorkspace`, and `AWSIAMProvisioner` directly inside Forge while keeping the package layout ready for a later move into `anvil`.
- **Local** (`local/`) — dispatcher plus the `launchagent` handler, the first workstation-only kind. Renders launchd plist XML, diffs against `$HOME/Library/LaunchAgents/<label>.plist`, writes atomically, and reloads via `launchctl`.

The cobra command shell lives in [`../reconcilecmd`](../reconcilecmd). `internal/reconcile/` stays focused on the reusable engine contract (`BuildPlan`, `Executor`, `RenderPlan`, `RenderApplyResult`) while `reconcilecmd` composes those pieces into `forge reconcile local|remote`.
