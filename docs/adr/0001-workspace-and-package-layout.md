# ADR 0001: Workspace and internal package layout

- **Status**: Accepted
- **Date**: 2026-04-18
- **Related**: [MK-4](https://linear.app/wiscotrashpanda/issue/MK-4/set-up-go-workspace-and-internal-package-structure)

## Context

Forge shipped its bootstrap CLI with three ad-hoc packages: `internal/cli`, `internal/ui`, and `internal/update`. That layout answered the questions the bootstrap phase needed to answer, but it did not commit to where new command domains should live, how they should plug into the cobra root, or where types destined for the future `alloy` module should sit in the meantime. Multiple domain tickets (workstation, manifest, reconcile, init, local) are queued up. Without a settled structure, each one would negotiate its own layout and the shell would drift quickly.

This ADR records the structural decisions made up front so later tickets inherit a single pattern rather than relitigating it per PR.

## Decisions

### 1. Initialize `go.work` with a single module

A `go.work` file at the repo root lists `./` as its only module. Carve-outs into sibling modules (most obviously `anvil` for reconciliation and `alloy` for shared schema) are on the roadmap, and retrofitting a workspace across in-flight branches is noisy. Declaring the workspace now is a one-file commit; today's build behavior is unchanged because single-module workspaces build identically to non-workspace single-module repos.

**Alternatives considered**

- *Defer `go.work` until a second module exists.* Rejected: MK-4 explicitly scopes workspace initialization, and the carve-outs are planned, not hypothetical. The cost of the file existing now is effectively zero.

### 2. Scaffold all five planned domains as README-only placeholders

`internal/workstation/`, `internal/manifest/`, `internal/reconcile/`, `internal/initcmd/`, and `internal/local/` are each created with a single `README.md` that names the domain and links back to MK-4. No Go source files are added.

**Alternatives considered**

- *Add a `doc.go` so each domain is a real Go package from day one.* Rejected: empty packages attract lint noise and create a false signal that there is code to review.
- *Lazy creation — only add a domain directory when its first command lands.* Rejected: MK-4 enumerates the layout, and co-locating the directory reservation with the layout decision keeps future PRs small and keeps reviewers from re-arguing directory names one domain at a time.

### 3. Rename the `init` domain directory to `initcmd`

The MK-4 proposed layout uses `internal/init/`. While `init` is a valid package name in Go, it is closely associated with the language's special `init()` function semantics and would be easy to misread in code review, docs, and IDE navigation. Using a directory named `init` with an inner `package initcmd` would also mismatch directory and package names and confuse tools and readers that assume the two line up. Renaming the directory to `initcmd/` and the package to `initcmd` avoids that ambiguity while keeping the filesystem and package naming consistent. The cobra command itself is still named `init`.

**Alternatives considered**

- *Keep directory `init/` and use `package initcmd` inside.* Rejected: directory/package name mismatches are a persistent source of confusion in `go doc`, IDE jump-to-definition, and `grep`.
- *Pick a different name such as `bootstrap`.* Rejected: the command is exposed as `forge init`, and the domain name should track the command name.

### 4. Reserve `pkg/` for alloy-bound exports

A top-level `pkg/` directory is created with a README explaining that it is the staging area for types and primitives intended to migrate to `alloy`. It is empty today. Naming the off-ramp up front gives reviewers a clear rule: alloy-candidate types live in `pkg/`, everything else lives in `internal/`.

**Alternatives considered**

- *Skip `pkg/` until a concrete alloy candidate exists.* Rejected: the repo-family boundary between `forge`, `anvil`, and `alloy` is already called out in `AGENTS.md`. Reserving the directory makes the boundary enforceable at review time.
- *Use `pkg/` as a generic public API surface.* Rejected: Forge intentionally keeps its CLI implementation unexported. A broader `pkg/` charter would blur the alloy-track story.

### 5. Domain registration via a single `Command()` constructor

Each domain package exports exactly one function, `Command() *cobra.Command`, that returns its fully configured cobra command with flags, subcommands, and a domain-owned `GroupID` constant. `internal/cli` imports each domain package and registers its command explicitly during root assembly.

**Alternatives considered**

- *Register commands via `init()` side effects in each domain package.* Rejected: implicit registration hides the wiring and makes the root command graph hard to reason about or test.
- *Let `internal/cli` construct each command directly, importing types from domain packages as needed.* Rejected: leaks domain details into the shell and forces every domain to negotiate its flag layout with the root.

### 6. Document the layout in `ARCHITECTURE.md` plus this ADR

`ARCHITECTURE.md` at the repo root is the operator-friendly reference for contributors adding commands. This ADR captures the alternatives and reasoning so the decisions are durable.

**Alternatives considered**

- *Fold everything into `AGENTS.md`.* Rejected: `AGENTS.md` is durable working guidance, not a structural reference. Mixing the two muddies both files.

## Consequences

- New command domains have a single documented pattern to follow and do not need to invent their own wiring.
- `go.work` exists before it is strictly required; contributors see a file that looks superfluous until the first carve-out. `ARCHITECTURE.md` explains why.
- The `init` → `initcmd` rename introduces a one-time divergence from the MK-4 ticket text. The Linear ticket description can be updated to match.
- Reserved but empty domain directories risk becoming stale if a planned domain is later dropped or merged. The domain READMEs link back to MK-4 so closing that ticket is a clear signal to revisit the reservation.
