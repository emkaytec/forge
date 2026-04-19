# Forge Architecture

This document describes the repository layout and the conventions future command domains follow. It is the canonical reference for contributors adding new commands. Durable working guidance for AI agents and contributors stays in [`AGENTS.md`](AGENTS.md); architectural reasoning with alternatives lives under [`docs/adr/`](docs/adr/).

## Repository layout

```
forge/
├── cmd/
│   └── forge/           # CLI entrypoint (main package)
├── internal/
│   ├── cli/             # product shell — cobra wiring, help rendering, flag parsing
│   ├── ui/              # shared styled output primitives (banner, palette, writers)
│   ├── update/          # self-update runtime used by the update command
│   ├── workstation/     # reserved: workstation setup domain (not yet implemented)
│   ├── manifest/        # reserved: manifest authoring/inspection domain (not yet implemented)
│   ├── reconcile/       # reserved: imperative reconciliation entrypoints (not yet implemented)
│   ├── initcmd/         # reserved: `forge init` domain (renamed from `init` to avoid confusion with Go's special `init()` semantics and keep directory/package naming unambiguous)
│   └── local/           # reserved: local development environment domain (not yet implemented)
├── pkg/                 # reserved: staging area for alloy-bound shared types (empty today)
├── docs/
│   └── adr/             # architecture decision records
├── go.work              # Go workspace (single module today; expandable for future carve-outs)
├── go.mod
├── ARCHITECTURE.md      # this file
├── AGENTS.md            # durable working guidance
└── README.md            # public product description
```

## Workspace file

Forge uses a Go workspace (`go.work`) at the repo root. Today it lists a single module (`./`), but it is declared now so future carve-outs into sibling modules — for example, extracting reconciliation into `anvil` or shared schema into `alloy` — can be added as additional `use` entries without restructuring the existing package tree.

Contributors do not need to set `GOWORK` manually. `go build`, `go test`, and `go vet` pick up the workspace automatically when invoked from the repo root.

## `internal/` versus `pkg/`

- **`internal/<domain>/`** is the default home for new Forge code. Everything that implements Forge's operator-facing CLI — command wiring, flag parsing, domain logic — lives here. The `internal/` prefix keeps these packages unimportable from outside the module, which is deliberate.
- **`pkg/`** is reserved for types and primitives that are candidates for future migration to the shared-schema `alloy` module. Anything placed here must be safe to expose as a public Go API and must follow the same public-safety rules as the rest of the repository. See [`pkg/README.md`](pkg/README.md) for the full policy.

If a new file could plausibly live in either directory, choose `internal/` until there is a concrete reason to promote it to `pkg/`.

## Command domains

Each operator-facing concern is a **command domain** — a directory under `internal/` that owns a single cobra command group plus its subcommands. The reserved domains today are `workstation`, `manifest`, `reconcile`, `initcmd`, and `local`. Each reserved directory currently contains only a `README.md`; the first implementation ticket for a domain replaces that README with real Go source files.

### Registration pattern

Every domain package exposes exactly one exported constructor:

```go
// Package workstation implements the `forge workstation` command group.
package workstation

import "github.com/spf13/cobra"

// GroupID is the cobra group that hosts workstation subcommands in help output.
const GroupID = "workstation"

// Command returns the configured workstation command group.
func Command() *cobra.Command {
    cmd := &cobra.Command{
        Use:     "workstation",
        Short:   "Manage local workstation setup.",
        GroupID: GroupID,
    }

    // cmd.AddCommand(newStatusCommand(), newApplyCommand(), ...)

    return cmd
}
```

`internal/cli` imports each domain package and registers its command during root assembly:

```go
// in internal/cli/root.go (illustrative)
root.AddGroup(&cobra.Group{ID: workstation.GroupID, Title: "Workstation Commands"})
root.AddCommand(workstation.Command())
```

Rules that the registration pattern pins down:

- A domain exports **one** `Command() *cobra.Command`. It does not export its subcommand constructors, flag structs, or internal helpers.
- A domain declares its own `GroupID` constant so renaming or splitting its group stays inside the domain.
- A domain does not register itself via `init()` side effects. The root command assembly lists every registered domain explicitly.
- `internal/cli` is the only package that imports domain packages for registration. Domains do not import each other.

Until a domain's first implementation ticket lands, its directory contains only a README and `internal/cli` does not import it. Help output is unchanged in the meantime.

## Existing non-domain packages

`internal/cli`, `internal/ui`, and `internal/update` predate the domain convention. They are product-shell concerns — the cobra wiring, shared styled output, and the self-update runtime — rather than operator-facing command domains, and they stay where they are. New operator-facing commands go into a domain package instead of extending these.

## Adding a new command

1. Pick the domain that owns the command. If none of the reserved domains fit, open a discussion before adding a new top-level directory.
2. Replace the domain's `README.md` with Go source files the first time real code lands. The `doc.go` file should carry the package comment; additional files contain the command, its subcommands, and any domain-local helpers.
3. Expose the `Command() *cobra.Command` constructor and a `GroupID` constant as described above.
4. Wire the domain into `internal/cli/root.go` by adding a cobra group and calling `root.AddCommand(<domain>.Command())`.
5. Update the durable repository docs that define or constrain the command's behavior. In practice that usually means `README.md`, `ARCHITECTURE.md`, and an ADR when the change introduces a meaningful architectural decision or trade-off.

## Future module carve-outs

When a domain is carved out into its own Go module — for example, reconciliation moving into a dedicated `anvil` module — the new module's path is appended to `go.work` as an additional `use` entry. Existing `internal/` packages continue to build without modification. The carve-out itself is out of scope for the current workspace setup; `go.work` exists today to make that later step mechanical.
