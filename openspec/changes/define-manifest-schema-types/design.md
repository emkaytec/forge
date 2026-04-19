## Context

Forge now has a reserved `manifest` command domain and a reserved `pkg/` area for alloy-candidate types, but it still lacks the schema contract that future manifest commands will rely on. MK-7 fills that gap by defining the first manifest envelope and four initial kinds: `github-repo`, `hcp-tf-workspace`, `aws-iam-provisioner`, and `launch-agent`.

This work sits on an important repo-family boundary. Forge owns operator-facing CLI flows, `anvil` owns reconciliation runtime behavior, and `alloy` owns the long-term shared schema boundary. In this repository, `pkg/` exists specifically as the temporary staging area for types that are expected to migrate into `alloy` later. That means the design needs to make the first implementation useful inside Forge without locking the project into a Forge-only schema shape.

The repository is also public and currently has no manifest dependency stack. Any implementation should stay sanitized, keep the field sets intentionally small, and avoid introducing a large schema framework just to decode a handful of YAML documents.

## Goals / Non-Goals

**Goals:**

- Define a stable manifest envelope for Forge with explicit `apiVersion`, `kind`, `metadata`, and typed `spec` handling.
- Provide first-pass Go types for the four MK-7 kinds in a layout that is easy to extract into `alloy` later.
- Support YAML marshal/unmarshal and schema-oriented validation for required fields, enums, and schedule shape.
- Keep the initial field sets deliberately narrow so they describe a reviewable v1 contract rather than a provider-parity aspiration.
- Capture the design rationale in an ADR that references the existing repo-family split and the intentional-scope guidance from `anvil`.

**Non-Goals:**

- Implement `forge manifest generate`, `forge manifest validate`, or any reconciliation/apply command in this change.
- Add provider clients, filesystem walking, remote API calls, or drift-correction behavior.
- Introduce general-purpose IAM modeling; `aws-iam-provisioner` stays limited to OIDC-backed provisioner roles.
- Add real manifests, secrets, account IDs, hostnames, or other environment-specific operational data.

## Decisions

### 1. Stage the new contract under `pkg/schema`

- **Chosen**: Add the manifest envelope, kind constants, and typed spec structs under `pkg/schema/`, with package comments calling out the alloy-candidate intent.
- **Why**: This matches Forge's documented `pkg/` policy: schema-oriented, publicly safe types that are expected to migrate into `alloy` later belong here rather than under `internal/manifest/`.
- **Alternatives considered**:
  - Add the types under `internal/manifest/`. Rejected because that would hide an intentionally shared contract inside a Forge-only package.
  - Add the types directly to `alloy` as part of this change. Rejected for this repo-scoped MK because the immediate deliverable is a Forge change set; keeping the code isolated under `pkg/schema` preserves a mechanical future extraction path without duplicating runtime concerns here.

### 2. Use a two-step decode path with explicit kind dispatch

- **Chosen**: Decode YAML into a small envelope first (`apiVersion`, `kind`, `metadata`, raw `spec`), then dispatch `spec` into the typed struct for the declared kind.
- **Why**: This keeps decoding explicit and easy to debug. It avoids reflection-heavy registries while still supporting kind-aware validation and round-trip tests.
- **Alternatives considered**:
  - Decode directly into `map[string]any`. Rejected because it weakens the shared contract and pushes type assertions downstream into every caller.
  - Introduce a generic registration framework for kinds. Rejected because the initial surface is small enough that a simple switch on known kinds is easier to review and extract later.

### 3. Keep validation schema-oriented and strict

- **Chosen**: Validate only schema concerns in this layer: supported `apiVersion`, supported `kind`, required fields, enum values, and cross-field schedule shape for `launch-agent`. Unknown fields should be rejected during YAML decoding rather than silently ignored.
- **Why**: This matches the intended `alloy` boundary from the anvil repo family: schema code can reject malformed manifests, but runtime/provider checks stay out of scope.
- **Alternatives considered**:
  - Permissive decode that ignores unknown fields. Rejected because it weakens the contract and makes it harder to know whether a manifest is truly supported.
  - Provider-aware validation such as checking GitHub repo names or HCP project existence. Rejected because those are runtime concerns, not schema concerns.

### 4. Model each kind as a narrow first-pass struct

- **Chosen**: Implement one typed spec struct per kind with the limited field set from MK-7, plus small enum/helper types where they improve clarity:
  - `github-repo`: `name`, `visibility`, `description`, `topics`, `default_branch`, `branch_protection`
  - `hcp-tf-workspace`: `name`, `organization`, `project`, `vcs_repo`, `execution_mode`, `terraform_version`
  - `aws-iam-provisioner`: `name`, `account_id`, `oidc_provider`, `oidc_subject`, `managed_policies`
  - `launch-agent`: `name`, `label`, `command`, `schedule`, `run_at_load`
- **Why**: The narrow surface matches the repo family's preference for intentional scoping over broad parity. It also keeps the future `alloy` API smaller and more defensible.
- **Alternatives considered**:
  - Expand fields now to cover more provider options. Rejected because the user has consistently preferred core-surface-first contracts.
  - Collapse all kinds into one generic schema struct. Rejected because it would make kind-specific validation and future extraction harder.

### 5. Use `gopkg.in/yaml.v3` and hand-written validation helpers

- **Chosen**: Add `gopkg.in/yaml.v3` for YAML support and keep validation in small hand-written methods/functions on the envelope and typed specs.
- **Why**: Go's standard library does not provide YAML decoding, but `yaml.v3` is the minimal, conventional dependency for this job. Hand-written validation keeps the behavior obvious without bringing in a schema DSL or reflection framework.
- **Alternatives considered**:
  - JSON-only parsing. Rejected because the manifest examples and intended operator workflows are YAML-based.
  - A heavier schema or validation framework. Rejected because it adds abstraction without solving a current problem.

### 6. Capture the boundary and scope in a new ADR

- **Chosen**: Add a new Forge ADR that explains why manifest schema starts in `pkg/schema`, why the initial kinds stay narrow, and why `aws-iam-provisioner` excludes general IAM management. The ADR should reference `anvil` ADR-0004 and ADR-0005 for prior repo-family context.
- **Why**: The design choices here are architectural, not just structural. Future changes will be easier to review if the rationale is recorded once.
- **Alternative considered**: Rely only on this OpenSpec change. Rejected because OpenSpec captures the proposed contract, while the ADR is the durable repository-level design record.

## Risks / Trade-offs

- **Forge/alloy drift before extraction** -> Keep all schema code isolated under `pkg/schema`, document the alloy-candidate intent in package comments, and pin behavior with tests so a future move is mechanical.
- **The first field sets may be too narrow for an immediate follow-on command** -> Accept that risk intentionally and expand with additive changes once a concrete operator workflow proves the need.
- **Strict decoding may reject manifests that future users expected to be tolerated** -> Treat that as a feature of the contract; add explicit fields or versioned changes instead of silently ignoring unknown input.
- **`launch-agent` schedule rules are the most shape-rich part of the initial schema** -> Keep the schedule model minimal (`interval` vs `calendar`) and validate only the fields needed to distinguish those modes.

## Migration Plan

1. Add `gopkg.in/yaml.v3` and create `pkg/schema/` with a package comment, kind constants, the envelope types, per-kind spec structs, and validation helpers.
2. Add focused tests that cover YAML unmarshal/marshal and validation for the envelope plus each supported kind.
3. Add the new ADR under `docs/adr/` and reference the relevant anvil ADRs for repo-family context.
4. Run `go test ./...` and `openspec validate define-manifest-schema-types --strict`.

Rollback is straightforward because no command surface depends on these types yet: remove `pkg/schema/`, the ADR, and the spec change before any follow-on command work lands.

## Open Questions

- None that block proposal/apply readiness. The implementation should follow the MK-7 manifest version and kind strings as written, and any future extraction into `alloy` can happen in a follow-on change once Forge's first manifest commands exist.
