# ADR 0004: Use Go for the Forge CLI

- **Status**: Accepted
- **Date**: 2026-04-23
- **Related**: Forge [ADR 0001](0001-workspace-and-package-layout.md), anvil ADR-0002

## Context

The primary implementation language options considered for Forge were Python and Go.

Python is the author's primary development language, with JavaScript and TypeScript also being familiar options. Python would have been a reasonable choice for CLI development and would likely have allowed fast iteration early on.

At the same time, Forge is intended to ship as a versioned operator-facing CLI artifact that runs predictably in GitHub Actions, cloud automation workflows, and local workstation setup flows. Forge also sits in a repo family where adjacent reconciliation and schema work already uses Go, and its command surface is expected to gather focused infrastructure automation entrypoints under one product shell.

## Decision

Forge will be implemented in Go.

## Rationale

- Go produces a simple executable binary, which is a strong fit for CI-driven installation, local operator workflows, and repeatable automation.
- A compiled binary simplifies release, distribution, and version pinning compared with a Python-based runtime and dependency environment.
- Using Go keeps Forge aligned with sibling infrastructure tooling and preserves a cleaner path for future shared packages, staged schema code, and intentional carve-outs.
- The command domains Forge is expected to host have natural overlap with infrastructure, platform, and control-plane-adjacent tooling, where Go is a common implementation language.
- The operational simplicity of a single deployable artifact matters more here than optimizing for the author's default scripting language.
- Using Go for this project is also a deliberate way to build deeper familiarity with a language that is widely used in infrastructure, platform, and cloud tooling.

## Consequences

### Positive

- Installation in GitHub Actions and local automation contexts can be straightforward and reproducible.
- Versioned binary releases fit the public-repo/private-configuration model cleanly.
- Strong typing and a compiled distribution model should help keep the CLI predictable as new command domains are added.
- The implementation language aligns with sibling repo-family tooling and common infrastructure automation patterns.
- The existing `cmd/<app>` and `internal/...` layout can remain lightweight while still leaving room for future module carve-outs.

### Negative

- The implementation language is not the author's primary day-to-day language, so development may be slower at first.
- Some early iteration tasks may be less fluid than they would be in Python.
- The project takes on the learning and maintenance cost of building in Go from the start.
- Small one-off automation tasks may require more ceremony than an equivalent short script.

## Alternatives Considered

### Python

Python would have been a defensible choice and likely the fastest path to an initial prototype. It was not selected because distribution as a standalone binary artifact, alignment with the repo family, and long-term fit with infrastructure tooling mattered more than short-term familiarity.
