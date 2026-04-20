# reconcile

Shared planning layer and target executors behind `forge reconcile local` and `forge reconcile remote`.

Package docs, including idempotency guarantees, compatibility filtering, and drift semantics per resource type, live in [`doc.go`](doc.go). Architectural rationale for the target split lives in [ADR 0003](../../docs/adr/0003-split-reconcile-by-execution-target.md).

- **Shared front half** (`compat.go`, `load.go`, `executor.go`, `plan.go`, `report.go`) — discovery, decode, validation, compatibility filtering, plan construction, and output rendering.
- **Remote** (`remote/`) — dispatcher plus one subpackage per remote-capable kind. Handlers are stub seams that return `ErrNotImplemented` from `Apply` today; MK-14 replaces them with real `anvil` delegation.
- **Local** (`local/`) — dispatcher plus the `launchagent` handler, the first workstation-only kind. Renders launchd plist XML, diffs against `$HOME/Library/LaunchAgents/<label>.plist`, writes atomically, and reloads via `launchctl`.

The cobra-command shell (`forge reconcile local|remote`) lives in [`internal/reconcilecmd`](../reconcilecmd/) and composes `BuildPlan`, `Executor`, `RenderPlan`, and `RenderApplyResult` from this package.
