# pkg

Staging area for types and primitives that are candidates for migration to the shared-schema `alloy` module.

## Why this directory exists

Forge is part of a repo family with deliberate boundaries:

- `forge` owns operator-facing CLI behavior.
- `anvil` owns reconciliation runtime behavior.
- `alloy` owns shared schema types, kind constants, and schema-oriented validation.

Code that logically belongs in `alloy` often needs to ship inside Forge first — the alloy carve-out is a future milestone, not a prerequisite. `pkg/` is the named parking spot for that code. Placing an alloy-candidate here makes the intent visible in code review and keeps the eventual migration mechanical.

## What belongs here

- Shared types, kind constants, or schema-oriented helpers that would logically live in `alloy` once it is carved out.
- Anything that other Forge packages reasonably need to import and that is safe to expose as a public Go API surface.

Each file should include a short comment noting its alloy-candidate status so future readers know it is expected to move.

## What does not belong here

- Forge-internal CLI wiring, command domains, or operator-facing helpers. Those live under `internal/<domain>/`.
- Code that contains real organization names, credentials, account IDs, or operational data. Everything here must satisfy the same public-safety rules as the rest of the public repository.
- Code that is not intended to migrate to `alloy`. If it is merely "shared between packages in Forge", it belongs under `internal/`.

## Current contents

This directory is reserved and intentionally empty. The first file lands when Forge introduces a type that warrants the alloy-candidate marking.
