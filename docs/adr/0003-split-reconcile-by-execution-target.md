# ADR 0003: Split reconcile by execution target

- **Status**: Accepted
- **Date**: 2026-04-19
- **Related**: Forge [ADR 0001](0001-workspace-and-package-layout.md), Forge [ADR 0002](0002-stage-manifest-schema-in-pkg.md), anvil ADR-0004

## Context

Forge now owns manifest authoring for both cloud-oriented and workstation-oriented resources. The first staged kinds already span both categories: `github-repo`, `hcp-tf-workspace`, `aws-iam-provisioner`, and `launch-agent`.

Those resources do not share one execution environment. Cloud-facing resources can be reconciled in remote automation such as GitHub Actions. Workstation resources such as launch agents must be reconciled on the local machine that will actually run them.

That creates a product-shape question for Forge: should manifest location define what is reconcilable, should the project split into separate engines, or should one product shell route the same manifest family to different execution targets?

This ADR records the v1 answer so future reconcile work does not have to re-argue whether local and remote resources need separate manifest systems or whether repo layout is part of the runtime contract.

## Decisions

### 1. Expose reconciliation as `forge reconcile local` and `forge reconcile remote`

Forge's operator-facing reconcile surface is split by execution target:

- `forge reconcile remote <path>`
- `forge reconcile local <path>`

The command split is about where work runs, not about maintaining two unrelated manifest systems. Both commands operate on the same manifest envelope and the same family of typed kinds.

This keeps the product shell explicit for operators while matching the practical runtime boundary: some resources are safe to apply in remote automation, and some are inherently machine-local.

### 2. Keep one manifest family and filter by supported kinds

Manifest files are not partitioned by repository identity as a hard contract. A manifest tree may contain both remote-capable and local-only kinds.

Each reconcile target loads manifests from the requested path, validates them, and then selects only the kinds it is allowed to execute. Kinds that do not belong to that target are either skipped with a clear report or rejected when strict mode is enabled.

The durable contract is kind-to-target compatibility, not "this repo may only contain cloud manifests" or "this directory may only contain local manifests."

Directory conventions such as `manifests/cloud/` and `manifests/local/` are still useful for organization and review, but they remain conventions rather than the primary enforcement mechanism.

### 3. Share load, validation, selection, and planning logic across reconcile targets

Local and remote reconciliation share a common front half:

- manifest discovery and decoding
- schema validation
- target compatibility checks
- selection and planning
- consistent dry-run and reporting behavior

Only the execution backend differs. Remote reconciliation hands work off to the cloud-capable reconciliation runtime. Local reconciliation invokes Forge-native local handlers for workstation resources.

This preserves one reviewable reconcile model without creating parallel engines that drift in manifest loading, filtering, or plan semantics.

### 4. Keep runtime ownership boundaries explicit

Forge owns the operator-facing CLI and target selection experience. Reconciliation runtime behavior for cloud resources continues to belong to `anvil` unless that code is intentionally migrated.

That means `forge reconcile remote` is a shell around the remote reconciliation runtime, not a duplicate implementation of it. `forge reconcile local` may host local-only execution code in Forge because those workstation handlers do not have an existing home in `anvil`.

### 5. Do not add placement metadata when kind semantics already answer the question

The first routing rule should come from manifest kind compatibility. A `launch-agent` manifest is local-only because the resource itself is machine-local. A GitHub repository manifest is remote-capable because its API-facing runtime can run in automation.

Forge should not add extra placement metadata merely to restate what the kind already implies. Additional execution metadata is only justified when one resource kind can genuinely run in more than one meaningful execution mode and operators need to choose between them.

## Alternatives Considered

- Make repository or directory location the runtime contract.
Rejected because it turns layout conventions into architecture, makes mixed manifest trees awkward, and couples reconciliation behavior to where files happen to live.

- Build separate local and remote manifest systems.
Rejected because it duplicates the envelope, loader, validation, and planning model even though the main difference is execution target rather than schema shape.

- Build a second local reconciliation engine by copying or re-implementing the remote one.
Rejected because it would create parallel behavior that is harder to review and keep aligned than a shared planning layer with different executors.

- Encode an execution target field in every manifest.
Rejected for v1 because many kinds already imply their valid target set. Repeating that in every manifest adds noise without increasing clarity.

## Consequences

- Forge gets one explicit reconcile UX that matches real execution environments without fragmenting manifest authoring.
- Mixed manifest trees remain possible, but operators can still organize files into cloud and local directories when that improves reviewability.
- Reconcile implementation work now has a clear package boundary: share the front half, swap executors underneath.
- If a future kind can run in both local and remote contexts, Forge may need narrowly scoped execution metadata or flags for that kind, but that choice can be made later without changing the overall command shape.
