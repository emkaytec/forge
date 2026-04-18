# Design: Scaffold Workspace Layout

## Context

The Forge bootstrap CLI currently ships as a single Go module with three internal packages: `internal/cli` (cobra wiring), `internal/ui` (styled output primitives), and `internal/update` (self-update). That is enough for today's help-first surface but does not encode where the next wave of command domains should land, how they should register themselves, or where types destined for `alloy` should sit in the meantime.

MK-4 is the structural commitment that fixes those answers before multiple domain tickets start landing in parallel. It is deliberately a scaffold — no real domain code is written here — because reserving the layout once is cheaper than rebasing five concurrent PRs on a mid-flight restructure.

The decisions below commit Forge to a workspace file, a directory layout, a reserved `pkg/` area, and a cobra registration pattern. Each is load-bearing for downstream tickets, so the alternatives are captured here rather than relitigated per PR.

## Decisions

### 1. Initialize `go.work` even with a single module

- **Chosen**: `go.work` at the repo root with one `use ./` entry today.
- **Alternative**: Defer `go.work` until the first carve-out actually adds a second module.
- **Why chosen**: MK-4 explicitly lists `go.work` initialization as in-scope, and the carve-outs into `anvil` or `alloy` are on the roadmap, not hypothetical. Adding `go.work` now is a one-line commit; retrofitting it later means coordinating a toolchain change across every open branch. The cost of the file existing early is effectively zero — single-module workspaces build identically to non-workspace single-module repos.
- **Trade-off accepted**: Contributors see a `go.work` file that looks superfluous until the first carve-out. `ARCHITECTURE.md` explains why it is there.

### 2. Scaffold all five planned domains as README-only placeholders

- **Chosen**: Create `internal/workstation/`, `internal/manifest/`, `internal/reconcile/`, `internal/initcmd/`, `internal/local/`, each containing a single `README.md` that names the domain and links to its owning MK ticket.
- **Alternatives considered**:
  - Scaffold each domain with a `doc.go` (`package workstation` etc.) so the Go toolchain sees a real package. Rejected — empty packages attract lint noise and create a false signal that there is code to review.
  - Lazy creation — only add a domain directory when its first command lands. Rejected — MK-4 explicitly enumerates the layout, and co-locating the directory reservation with the layout decision keeps future PRs smaller and keeps reviewers from re-arguing directory names one domain at a time.
- **Why chosen**: README-only placeholders declare intent without creating stale Go code. When a domain's first ticket (e.g. MK-5 for workstation) lands, the author replaces the README with real source files and the directory already exists in history.

### 3. Rename the `init` domain directory to `initcmd`

- **Chosen**: Directory name `internal/initcmd/`, package name `initcmd`.
- **Context**: Go reserves the identifier `init` for package initializer functions. A directory named `init` is syntactically legal, but `package init` is not — the file would have to declare `package initcmd` or similar, creating a directory/package name mismatch that confuses `go doc`, IDE jump-to-definition, and readers grepping for the package.
- **Trade-off accepted**: The Linear ticket's proposed layout uses `init/`. This proposal intentionally diverges and calls it out so the ticket description can be updated or cross-referenced.

### 4. Reserve `pkg/` for future alloy migration only

- **Chosen**: `pkg/` at the repo root with a README explaining its role as the staging area for types and primitives that are candidates for migration into the shared-schema `alloy` module.
- **Alternatives considered**:
  - Skip `pkg/` until a concrete alloy candidate exists. Rejected — the directory is load-bearing in the repo-family boundary story (see `AGENTS.md`'s "Repo Family Boundary"), and naming it up front lets reviewers push back on code that should live in `pkg/` but was written under `internal/`.
  - Use `pkg/` as a generic public-API surface. Rejected — Forge intentionally keeps its CLI implementation unexported. A broader `pkg/` charter would blur the alloy-track story.
- **Why chosen**: Naming the off-ramp is cheap and sets a clear rule for future reviewers: if a type looks like it belongs in alloy, it goes in `pkg/` first.

### 5. Cobra registration: one `Command()` constructor per domain

- **Chosen**: Each domain package exposes a single exported function, `Command() *cobra.Command`, that returns a fully configured cobra command (name, short/long description, flags, subcommands, and a `GroupID`). `internal/cli` imports each domain package and calls `Command()` during root assembly. The `GroupID` constant is declared by the domain package, not by `internal/cli`, so renaming or splitting a group stays inside the domain.
- **Alternatives considered**:
  - Register commands via `init()` side effects in each domain package. Rejected — implicit registration hides the wiring and makes the root command graph hard to reason about or test.
  - Have `internal/cli` construct each command directly, importing types from domain packages as needed. Rejected — leaks domain details into `internal/cli` and forces every domain to negotiate its flag layout with the root.
- **Trade-off accepted**: Domain packages take a hard dependency on cobra. This is already true of `internal/cli` today, and the `build-cli-shell` proposal (archived) committed the repo to cobra as the dispatch layer.

### 6. Documentation lives in `ARCHITECTURE.md` plus an ADR

- **Chosen**: A new root-level `ARCHITECTURE.md` summarizes the layout and registration pattern in operator-friendly terms, and a new ADR under `docs/adr/` captures the decisions and alternatives for future reviewers.
- **Alternative**: Fold everything into `AGENTS.md`. Rejected — `AGENTS.md` is durable working guidance, not a structural reference document. Mixing the two muddies both.
- **Why chosen**: `ARCHITECTURE.md` is the file a new contributor looks for; the ADR is the file a future reviewer looks for when they want to know *why* the structure is what it is.

## Out of Scope

- Writing real code for any domain package. That is each domain's own MK ticket.
- Refactoring existing `internal/cli`, `internal/ui`, `internal/update` into domain packages. Those are product-shell concerns, not domain concerns, and stay where they are.
- Adding a second Go module. `go.work` is initialized as a single-entry workspace; carve-outs happen in a later change.
- Touching the `demo` command. It stays registered directly by `internal/cli`; converting it to the domain pattern is optional cleanup, not part of this change.

## Risks

- **Dead directory risk.** If a planned domain is later dropped (e.g. MK decides `local` folds into `workstation`), the reserved directory becomes misleading. Mitigation: the domain READMEs link to their owning MK ticket, so closing that ticket is a clear signal to remove or rename the directory.
- **Pattern drift risk.** Once one domain lands, reviewers need to keep new domains honest to the `Command()` contract. Mitigation: the `forge-workspace-layout` spec's scenarios pin the contract in a way that can be verified in code review.
