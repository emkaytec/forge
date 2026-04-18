# Forge OpenSpec

This directory holds the OpenSpec artifacts for Forge.

The layout follows the current upstream OpenSpec project structure:

- `config.yaml` contains optional project-level OpenSpec defaults and rules.
- `specs/` is the source of truth for current Forge behavior.
- `changes/` holds proposed changes before they are implemented and archived.

## Conventions

- Keep specs behavior-focused. Describe what operators can observe rather than how the Go code is organized internally.
- Use RFC 2119 language such as `SHALL`, `SHOULD`, and `MAY` in requirements.
- Prefer Given/When/Then scenarios for behavior that should be easy to test or validate.
- Keep Forge product boundaries explicit. Reconciliation behavior belongs in `anvil`, and shared schema or validation belongs in `alloy` unless intentionally migrated.
- Keep all examples sanitized. Do not place real organization names, credentials, account IDs, hostnames, or other operational data in this tree.

## Starting Point

The initial baseline spec captures the current bootstrap CLI behavior that already exists in this repository. Future work can add new capability specs or change folders as Forge grows.
