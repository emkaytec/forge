# ADR 0005: Align manifest authoring with Anvil Terraform

- **Status**: Accepted
- **Date**: 2026-04-25
- **Related**: MK-22, ADR 0002, ADR 0003

## Context

Forge originally staged its own manifest schema and reconciliation command surface. That was useful while the repo family explored where authoring, schema, and runtime behavior should live, but the active baseline architecture workflow has moved to Anvil's Terraform composition layer.

Anvil now reads `GitHubRepository` YAML from the root `.forge/` directory and translates those manifests into Terraform module calls. Keeping Forge's older custom manifest generators, multi-resource compose workflow, and public reconcile command would leave operators with two competing contracts for the same desired state.

## Decision

Forge will author the Anvil Terraform manifest shape directly for the current MVP.

The public manifest command surface is narrowed to:

- `forge manifest generate github-repo`
- `forge manifest validate`

`forge manifest compose` is removed. The public `forge reconcile` command is removed. The first generator writes one Anvil-compatible `GitHubRepository` manifest to `.forge/<name>.yaml` using:

- `apiVersion: anvil.emkaytec.dev/v1alpha1`
- `kind: GitHubRepository`
- `spec.repository`
- optional `spec.createTerraformWorkspaces`, `spec.environments`, and `spec.workspace` when Terraform resources are enabled

The interactive prompt flow asks for only the MVP inputs: repository name, whether this is a Terraform repository, and environment/account details when Terraform workspace resources are requested. Optional repository and workspace details remain available as flags so the prompt does not try to cover every edge case up front.

## Consequences

Forge has one authoring contract for the active baseline workflow, and generated manifests can be consumed directly by Anvil.

The earlier staged schema and reconcile runtime remain internal implementation history for now. They are no longer part of the public CLI surface and can be migrated or deleted deliberately in a later cleanup.

Future manifest generators should be added one Terraform module/resource at a time instead of reviving generic composition or plugin infrastructure.
