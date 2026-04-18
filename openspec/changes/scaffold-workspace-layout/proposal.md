# Scaffold Workspace Layout

## Why

MK-4 asks Forge to commit to a long-lived Go workspace and internal package layout before the first domain commands (workstation, manifest, reconcile, init, local) land. Today the repo is a single Go module with `internal/{cli, ui, update}` packages that grew organically during bootstrap. That layout worked for a help-first CLI but does not yet answer three questions that every future ticket (MK-5, MK-8, MK-11, MK-12, and beyond) will ask:

- Where does a new command domain live?
- How does a new domain plug into the root command without each author reinventing the wiring?
- Where do types that should eventually migrate to `alloy` live while they still ship in Forge?

Fixing the answer once, before domain code arrives, keeps the Forge product shell coherent and keeps the door open for later carve-outs into sibling repositories without a structural rewrite.

## What Changes

- Initialize `go.work` at the repo root listing the current module. Today the workspace has one entry; tomorrow it can grow as additional modules are carved out.
- Reserve a directory per planned command domain under `internal/`: `workstation/`, `manifest/`, `reconcile/`, `initcmd/`, `local/`. Each is a README-only placeholder until the owning ticket lands real code. The `init` directory from the MK-4 notes becomes `initcmd/` because `init` is a reserved identifier for Go package initializer functions and cannot be used as a package name.
- Reserve `pkg/` at the repo root with a README describing it as the staging area for types and primitives intended to migrate to the shared-schema `alloy` module.
- Document the cobra registration pattern in a new `ARCHITECTURE.md` at the repo root: each domain package exposes a single `Command() *cobra.Command` constructor, owns its `GroupID` constant, and is imported by `internal/cli` during root command assembly.
- Capture the layout decision in an ADR under `docs/adr/` so the alternatives and trade-offs survive beyond this change folder.
- ADD a new `forge-workspace-layout` capability spec covering the public contract that future domains must follow: domain-driven registration, reserved directories, and the single-module workspace.

No user-visible CLI behavior changes. `forge --help`, `forge --version`, and the existing `demo` command continue to render identically until a domain registers its first real subcommand.

## Impact

- Affected capabilities: new `forge-workspace-layout`. No modifications to `forge-cli-bootstrap`, `cli-release-workflow`, or `self-update`.
- Affected files (future apply stage): new `go.work`, new `internal/<domain>/README.md` (5 domains), new `pkg/README.md`, new `ARCHITECTURE.md`, new ADR under `docs/adr/`.
- Unblocks downstream MK tickets that add domain commands by giving each one a single pattern to follow.
- Keeps the Forge / anvil / alloy boundary explicit: `pkg/` is the named off-ramp for alloy-bound types, and `go.work` is the named on-ramp for future anvil carve-outs.
