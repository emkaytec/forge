# Forge Workspace Layout Specification Delta

## ADDED Requirements

### Requirement: Single-module Go workspace

The Forge repository SHALL declare a Go workspace via a `go.work` file at the repo root so future module carve-outs can be added without restructuring existing packages.

#### Scenario: Clean checkout builds within the workspace

- GIVEN a clean checkout of the repository
- WHEN a contributor runs `go build ./...` from the repo root
- THEN the build succeeds using the workspace defined in `go.work`
- AND no additional `GOWORK` environment configuration is required

#### Scenario: Workspace lists the primary module

- GIVEN the repository contains a single Go module at the repo root
- WHEN the `go.work` file is inspected
- THEN it contains a `use ./` entry for that module
- AND no other `use` entries are present

#### Scenario: Future module carve-out

- GIVEN a future decision carves an existing `internal/` domain into its own Go module under a new subdirectory
- WHEN the new module is added
- THEN `go.work` gains a `use` entry for the new module path
- AND existing `internal/` packages continue to build without modification

### Requirement: Domain-driven command registration

The Forge CLI SHALL compose its command surface by importing a single exported `Command() *cobra.Command` constructor from each domain package under `internal/<domain>/`.

#### Scenario: Domain contributes a grouped command

- GIVEN a domain package at `internal/<domain>/` that exposes `Command() *cobra.Command`
- WHEN `internal/cli` builds the root command
- THEN it registers the domain's command by calling that constructor exactly once
- AND the registered command carries a `GroupID` declared as a constant by the domain package
- AND the domain's subcommands appear under that group in `forge --help`

#### Scenario: Domain without a registered constructor

- GIVEN a domain directory has been reserved under `internal/` but no `Command()` constructor is yet registered with the root
- WHEN the Forge CLI renders help output
- THEN no group or command appears for that domain
- AND help output matches the pre-scaffold baseline

#### Scenario: Root does not reach inside the domain package

- GIVEN a domain package with internal helpers, flag types, and subcommands
- WHEN `internal/cli` assembles the root command
- THEN it references only the domain's `Command()` constructor and `GroupID` constant
- AND it does not import domain-internal helper types or flag structs

### Requirement: Reserved top-level directories

The Forge repository SHALL keep command-domain implementations under `internal/<domain>/` and SHALL reserve `pkg/` for exports intended for migration to the shared-schema `alloy` module.

#### Scenario: Planned command domain adds code

- GIVEN a planned domain (such as `workstation`, `manifest`, `reconcile`, `initcmd`, or `local`) is being implemented
- WHEN new Go source files are added for that domain
- THEN the files live under `internal/<domain>/`
- AND no new top-level directories are introduced solely for that domain

#### Scenario: A shared type is an alloy candidate

- GIVEN a type, kind constant, or schema-oriented helper is a candidate for future migration to the `alloy` repository
- WHEN it is first introduced in Forge
- THEN it lives under `pkg/`
- AND a comment in its owning file documents the alloy-candidate intent

#### Scenario: `pkg/` stays public-safe

- GIVEN code is placed under `pkg/`
- WHEN the repository is built or published
- THEN the code contains no real organization names, credentials, account IDs, or operational data
- AND it follows the same public-safety rules as the rest of the public repository

### Requirement: Documented layout reference

The Forge repository SHALL provide an `ARCHITECTURE.md` at the repo root that describes the workspace file, the `internal/<domain>/` layout, the `pkg/` reservation, and the `Command() *cobra.Command` registration pattern.

#### Scenario: Contributor looks up the layout

- GIVEN a new contributor is adding a command domain
- WHEN they open `ARCHITECTURE.md`
- THEN they find the required domain directory convention
- AND they find the required `Command()` constructor contract
- AND they find guidance on when to use `pkg/` versus `internal/`

#### Scenario: Layout decision has a durable record

- GIVEN a reviewer wants to understand why the workspace was initialized with a single module, or why `init` was renamed to `initcmd`
- WHEN they search `docs/adr/`
- THEN they find an ADR capturing the alternatives considered and the reasoning for the chosen layout
