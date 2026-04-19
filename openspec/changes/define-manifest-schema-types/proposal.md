## Why

MK-7 is the first Forge change that needs a shared manifest contract before the `manifest` and `reconcile` command domains can grow. Right now Forge has a reserved `pkg/` boundary for alloy-candidate types, but no concrete manifest envelope or resource schemas, which means `generate`, `validate`, and future reconcile entrypoints would each be forced to invent their own shape.

Defining a small, explicit manifest schema set now gives Forge one reviewable contract for initial authoring and inspection work while still keeping reconciliation behavior in `anvil` and preserving a clean future extraction path into `alloy`.

## What Changes

- Add a top-level manifest envelope type for Forge manifests with explicit `apiVersion`, `kind`, `metadata`, and typed `spec` handling.
- Add initial alloy-candidate schema types under `pkg/` for four manifest kinds: `github-repo`, `hcp-tf-workspace`, `aws-iam-provisioner`, and `launch-agent`.
- Add kind constants and YAML encoding/decoding behavior that let Forge load sanitized manifest files into the correct typed schema without introducing provider apply logic.
- Add an ADR that records why these schemas land in Forge's `pkg/` staging area first, why the initial field sets stay intentionally narrow, and why `aws-iam-provisioner` is scoped only to OIDC-backed provisioner roles.
- Add OpenSpec coverage for the new manifest schema contract so future command work can build on a reviewed baseline.
- Do not add reconciliation flows, provider clients, secrets, or environment-specific operational data as part of this change.

## Capabilities

### New Capabilities
- `manifest-schema-types`: Defines the initial Forge manifest envelope, supported kinds, and YAML-backed schema contracts for `github-repo`, `hcp-tf-workspace`, `aws-iam-provisioner`, and `launch-agent`.

### Modified Capabilities
- None.

## Impact

- Affected code: new alloy-candidate schema packages under `pkg/`, YAML round-trip tests, and a new ADR under `docs/adr/`.
- Affected future command work: `forge manifest generate`, `forge manifest validate`, and future reconcile-oriented entrypoints gain a shared manifest contract instead of each inventing their own shape.
- Affected boundaries: Forge gains schema staging code that is explicitly intended to migrate to `alloy`; reconciliation behavior remains out of scope for this change.
