# ADR 0002: Stage manifest schema in pkg/schema

- **Status**: Accepted
- **Date**: 2026-04-19
- **Related**: Forge [ADR 0001](0001-workspace-and-package-layout.md), anvil ADR-0004, anvil ADR-0005

## Context

Forge's bootstrap repo layout already reserved `pkg/` as the staging area for public, schema-oriented types that are expected to migrate into `alloy` later. MK-7 is the first Forge change that actually needs that off-ramp: future `manifest` commands need one typed, reviewable contract for YAML manifests, but the long-term shared-schema home still belongs to `alloy`, not to Forge-specific command packages.

The initial manifest surface also carries scope pressure. GitHub repositories, HCP Terraform workspaces, AWS IAM roles, and macOS launch agents all have much broader real-world configuration surfaces than Forge wants to own on day one. Shipping an expansive schema now would blur the boundary between schema staging and provider/runtime behavior before Forge has even added the first operator-facing manifest command.

This ADR records the narrow first pass so later changes do not have to re-argue where the schema lives, how broad it should be, or why the AWS IAM shape is intentionally constrained.

## Decisions

### 1. Stage the first manifest contract in `pkg/schema`

Forge keeps the manifest envelope, supported kind constants, typed spec structs, and schema-only validation helpers in `pkg/schema`.

This follows the package boundary set in ADR 0001: `pkg/` is not a general public API bucket, it is the temporary parking area for alloy-candidate exports. Putting the schema in `internal/manifest` would make a shared contract look Forge-private, while moving it into `alloy` immediately would turn a Forge-scoped change into a cross-repo migration before the first command consumer exists.

### 2. Keep the v1 manifest envelope explicit and strict

The initial envelope is a small YAML contract with `apiVersion`, `kind`, `metadata`, and `spec`. Forge decodes the envelope first, validates the declared version and kind, and then dispatches `spec` into the typed schema for the supported kind.

Unknown fields are rejected during YAML decoding instead of being silently tolerated. The schema layer is allowed to answer only schema questions: supported version/kind, required fields, enum values, and simple cross-field shape validation.

### 3. Ship narrow first-pass kinds instead of broad provider parity

The initial kinds are:

- `GitHubRepository`
- `HCPTerraformWorkspace`
- `AWSIAMProvisioner`
- `LaunchAgent`

Each kind intentionally exposes only a small field set. That keeps the contract reviewable, aligns with Forge's "scope intentionally" guidance, and preserves additive expansion later when a real operator workflow proves the need.

This mirrors the repo-family direction already captured in anvil ADR-0005: manage the core surface first and expand deliberately rather than chasing parity up front.

### 4. Keep `AWSIAMProvisioner` limited to OIDC-backed provisioner roles

The AWS IAM schema is not a general IAM role model. It covers only the fields needed to describe OIDC-backed provisioner roles: role naming, target account, OIDC provider identity, subject matching, and attached managed policies.

General IAM features such as arbitrary trust policy modeling, inline policies, permissions boundaries, and unrelated role settings stay out of scope. That boundary keeps Forge's schema staging code public-safe and prevents this repo from quietly turning into a generic IAM authoring framework.

This is consistent with the repo-family split described in anvil ADR-0004: Forge owns operator-facing workflows, `anvil` owns reconciliation/runtime behavior, and `alloy` owns the eventual shared schema boundary.

## Alternatives Considered

- Put the schema under `internal/manifest`.
Rejected because it hides a deliberately shared contract in a Forge-only package.

- Move the schema into `alloy` immediately.
Rejected because MK-7 is a Forge change and there is not yet a command surface that justifies the cross-repo migration cost.

- Start with broader provider-specific field coverage.
Rejected because the first value of this work is a stable envelope and a narrow typed baseline, not exhaustive provider modeling.

## Consequences

- Forge now has a concrete manifest contract that future `manifest` commands can consume without inventing their own YAML shape.
- The schema is isolated in a way that makes a later move into `alloy` mostly mechanical.
- Some legitimate real-world configuration needs are intentionally deferred until a follow-on change justifies broadening a specific kind.
- Reviewers have a durable record that strict decoding and narrow field sets are product decisions, not temporary omissions.
