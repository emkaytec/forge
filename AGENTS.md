# Forge

## Product Overview

Forge is the operator-facing umbrella CLI for imperative automation across cloud infrastructure, DevOps workflows, and local development environment setup.

It is a Go CLI intended to gather focused automation entrypoints under one product shell while preserving clear boundaries around shared schema, reconciliation behavior, and private operational data.

Forge is not a background service, a generic plugin platform, or a mandate to collapse every sibling repository into one code path immediately.

## Core Principles

- Keep command behavior explicit, readable, and easy to debug.
- Prefer practical automation flows over abstraction-heavy framework design.
- Preserve deliberate boundaries between product shell, reconciliation code, and shared schema ownership.
- Keep public repository contents sanitized and safe for portfolio use.
- Add scope intentionally rather than chasing provider or workflow parity all at once.
- Favor standard library solutions first unless a dependency is clearly justified.

## Repo Family Boundary

Forge is the umbrella product shell, but the current repo family boundaries still matter.

- `forge` owns the top-level CLI experience and imperative automation workflows that belong under the Forge product.
- `anvil` owns reconciliation runtime behavior unless code is intentionally migrated.
- `alloy` owns shared schema types, kind constants, and schema-oriented validation.
- Higher-level composition or migration should not silently blur those boundaries.

If shared types or validation rules are needed, add them to `alloy` first instead of recreating them locally in `forge`.

## V1 Scope

The bootstrap phase of Forge is intentionally small.

Initial expected scope:

- a lightweight Go CLI entrypoint
- room for imperative automation commands spanning cloud, DevOps, and workstation setup
- public-facing docs that explain the product boundary clearly
- ADR scaffolding for durable architectural decisions

## Explicit Non-Goals

Forge bootstrap does not include:

- a finalized monorepo migration
- copied schema definitions that belong in `alloy`
- duplicated reconciliation logic from `anvil`
- embedded real manifests, secrets, or environment-specific operational data
- speculative framework or plugin infrastructure

## Public Repository Boundary

This repository is intended to remain public.

- Public examples and documentation must use sanitized placeholder values.
- The repository must never include real organization names, repository names, credentials, secrets, account IDs, or operational values unless the value is already intentionally public and harmless.
- Real implementation data belongs in separate private repositories or local private configuration.

## Coding Patterns

- Follow the lightweight Go layout already used in sibling repositories: `cmd/<app>` for the entrypoint and `internal/...` for application code.
- See [`ARCHITECTURE.md`](ARCHITECTURE.md) for the canonical workspace layout, the `internal/<domain>/` convention, the `pkg/` reservation, and the cobra `Command() *cobra.Command` registration pattern that new command domains follow.
- Prefer direct command handling over deep abstraction.
- Keep the bootstrap easy to evolve rather than prematurely optimizing the package structure.
- Add tests when they meaningfully pin down command behavior or tricky logic.
- Add ADRs under `docs/adr/` when a decision has meaningful alternatives or trade-offs.

## Working Style

- Keep durable project guidance in this file.
- Keep `README.md` public-facing and concise.
- Prefer direct implementation work over process-heavy planning artifacts.
- When the product boundary changes, update both `README.md` and this file so the public story and internal guidance stay aligned.
