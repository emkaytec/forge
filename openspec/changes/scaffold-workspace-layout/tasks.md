# Tasks: Scaffold Workspace Layout

## 1. Workspace file
- [x] 1.1 Add `go.work` at the repo root declaring `go 1.26` and a single `use ./` entry
- [x] 1.2 Run `go build ./...` from the repo root and confirm it succeeds under the workspace

## 2. Reserve domain directories
- [x] 2.1 Create `internal/workstation/README.md` naming the domain and linking to its owning MK ticket
- [x] 2.2 Create `internal/manifest/README.md` naming the domain and linking to its owning MK ticket
- [x] 2.3 Create `internal/reconcile/README.md` naming the domain and linking to its owning MK ticket
- [x] 2.4 Create `internal/initcmd/README.md` naming the domain, noting the rename from `init`, and linking to its owning MK ticket
- [x] 2.5 Create `internal/local/README.md` naming the domain and linking to its owning MK ticket

## 3. Reserve `pkg/` for alloy-bound exports
- [x] 3.1 Create `pkg/README.md` describing the directory as the staging area for types that will migrate to `alloy`
- [x] 3.2 Note in the README that code placed here must stay public-safe and free of operational data

## 4. Document the cobra registration pattern
- [x] 4.1 Add `ARCHITECTURE.md` at the repo root covering: workspace file, `internal/` domain layout, `pkg/` reservation, `Command() *cobra.Command` constructor contract, domain-owned `GroupID` constants
- [x] 4.2 Include a minimal code snippet in `ARCHITECTURE.md` showing the expected `Command()` signature and how `internal/cli` will call it
- [x] 4.3 Add an ADR under `docs/adr/` (next available number) capturing: the workspace-now decision, the README-only domain scaffolds, the `init`→`initcmd` rename, and the rejected alternatives

## 5. Cross-reference existing guidance
- [x] 5.1 Update `AGENTS.md`'s "Coding Patterns" section to point at `ARCHITECTURE.md` for the canonical layout rules
- [x] 5.2 Confirm `README.md` still matches the product story; update only if the workspace / `pkg/` story needs a one-line mention

## 6. Validation
- [x] 6.1 `go build ./...` passes
- [x] 6.2 `go vet ./...` passes
- [x] 6.3 `go test ./...` passes (no test changes expected; existing tests must still run)
- [x] 6.4 `forge --help` output is byte-identical to the pre-change baseline
- [x] 6.5 `openspec validate scaffold-workspace-layout --strict` passes
