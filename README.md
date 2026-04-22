# Forge

Forge is a Go CLI for imperative automations across cloud infrastructure, DevOps workflows, and local development environment setup.

This repository is the public product repository for Forge. It is intended to show the product direction, implementation patterns, and documentation without exposing real production data.

## Status

This repository is an initial working scaffold.

The current bootstrap establishes the public project shape, a lightweight Go CLI entrypoint, the first manifest authoring and validation workflow, and the durable guidance/docs structure the repository will build on.

Forge is intended to act as the umbrella CLI in the broader repo family. That does not erase the existing roles of sibling projects overnight:

- `forge` is the product shell for operator-facing imperative automation
- `anvil` remains the reconciliation engine boundary unless and until code is intentionally moved
- `alloy` remains the shared schema and validation boundary

## Core Principles

- Keep the CLI lightweight, explicit, and easy to debug.
- Prefer practical operator workflows over framework-heavy abstractions.
- Preserve clear repo and package boundaries while the product shape evolves.
- Keep public code and docs sanitized so real operational data stays private.
- Start with a small intentional surface and expand only when the need is clear.

## V1 Direction

Forge v1 is expected to provide a focused CLI for imperative engineering tasks that do not fit neatly into declarative infrastructure or one-off shell scripts.

Initial areas of interest:

- cloud infrastructure helper workflows
- DevOps and release workflow helpers
- local development environment setup helpers

## Non-Goals

Forge bootstrap does not yet try to:

- collapse `anvil`, `alloy`, and future authoring packages into one root package
- define the final monorepo migration plan
- embed real environment-specific data or operational identifiers
- introduce plugin systems or framework-heavy scaffolding before they are needed

## Public Repository Boundary

This repository is intended to remain public.

- Public documentation and examples must use sanitized placeholder values.
- The repository must never include real credentials, secrets, account IDs, hostnames, or operational values.
- Real manifests, inventories, and environment-specific configuration belong in separate private repositories or local private data stores.

## Local Development

Run the CLI help locally with:

```bash
go run ./cmd/forge --help
```

Generate a starter manifest in the current directory with:

```bash
go run ./cmd/forge manifest generate launch-agent brew-update
```

Validate one manifest file or every manifest in a directory with:

```bash
go run ./cmd/forge manifest validate ./examples
```

Run the current test suite with:

```bash
go test ./...
```

Build a local binary with:

```bash
go build -o bin/forge ./cmd/forge
./bin/forge --help
```

## Manifest Workflows

Forge now ships a `manifest` command domain for starter manifest authoring and schema validation.

- `forge manifest compose <blueprint> [application]`
- `forge manifest generate github-repo <name>`
- `forge manifest generate hcp-tf-workspace <vcs-repo>`
- `forge manifest generate aws-iam-provisioner <vcs-repo>`
- `forge manifest generate launch-agent <name>`
- `forge manifest validate <file-or-directory>`

`forge manifest generate ...` writes one primitive manifest at a time. `forge manifest compose ...` is the higher-level authoring layer for workflows that need to emit several primitive manifests from one prompt flow.

`forge manifest compose terraform-github-repo` starts with the same repo inputs as `forge manifest generate github-repo`, then prompts for one or more deployment environments, the AWS account for each selected environment, and the shared HCP Terraform plus IAM settings needed to fan out a full repo stack. It writes:

- `<application>/github-repo.yaml`
- `<application>/hcp-tf-workspace-<env>.yml` for each selected environment
- `<application>/aws-iam-provisioner-<env>-gha.yaml` and `<application>/aws-iam-provisioner-<env>-tfc.yaml` for each selected environment

Generated manifests write `<directory>/<resource>.yaml` under the current directory by default. `github-repo` uses the application name for the shared directory while keeping `metadata.name` owner-scoped. `hcp-tf-workspace` writes `hcp-tf-workspace-<env>.yml`, uses the repository name for the shared directory, and keeps `metadata.name` owner-scoped. `aws-iam-provisioner` uses the repository name for the shared directory, always writes both `aws-iam-provisioner-<env>-gha.yaml` and `aws-iam-provisioner-<env>-tfc.yaml`, keeps `metadata.name` owner-scoped, and uses the repository name for `spec.name`. Pass `--dir <relative-path>` to place generated files under a different relative directory.

The launch-agent example in [examples/brew-update.yaml](examples/brew-update.yaml) shows the manifest-driven local automation pattern currently favored in Forge instead of a bespoke `forge local` workflow.

## Workstation Workflows

Forge now ships a `workstation` command domain for day-two workstation operations across tagged AWS and GCP instances.

- `forge workstation list`
- `forge workstation start <name>`
- `forge workstation stop <name>`
- `forge workstation connect <name>`
- `forge workstation reload-config [name]`

`forge workstation list` discovers instances using the shared conventions called out in MK-5:

- AWS tags: `forge-managed=true` and `forge-role=workstation`
- GCP labels: `forge-managed=true` and `forge-role=workstation`

`connect` uses the workstation's Tailscale hostname. Forge will read that hostname from cloud metadata when present, and it can also be supplied in a local config file at `~/.config/forge/config.yaml`:

```yaml
workstations:
  - name: forge-dev
    provider: aws
    tailscale_hostname: forge-dev.tailnet.ts.net

ansible:
  repo_path: ~/Code/private/forge-config
  inventory: inventory/hosts.yaml
  playbook: playbooks/workstation.yaml
```

`reload-config` assumes the Ansible repo already exists locally. Forge is only the trigger. By default it expects:

- an inventory at `inventory/hosts.yml` or `inventory/hosts.yaml`
- a workstation playbook at `playbooks/workstation.yml` or `playbooks/workstation.yaml`
- inventory host or group names that match the Forge workstation name used with `reload-config <name>`

The Ansible repo path can be configured with `ansible.repo_path` in the config file above or with `FORGE_ANSIBLE_REPO`. `FORGE_ANSIBLE_INVENTORY` and `FORGE_ANSIBLE_PLAYBOOK` override the default inventory and playbook paths when needed.

## Init Workflows

Forge now ships an `init` command domain for one-time bootstrap work that is easier to do imperatively than declaratively.

- `forge init aws-oidc`
- `forge init aws-oidc --account-id 123456789012`

`forge init aws-oidc` uses the ambient AWS session to resolve the target account when `--account-id` is omitted, prints that account ID at the start of the run, and then ensures these shared IAM OIDC providers exist:

- GitHub Actions at `https://token.actions.githubusercontent.com` with audience `sts.amazonaws.com`
- HCP Terraform at `https://app.terraform.io` with audience `aws.workload.identity`

The command is idempotent. Re-running it reports whether each provider was created or already existed. It only bootstraps the identity providers; IAM roles and attached policies remain managed through `AWSIAMProvisioner` manifests.

## Reconcile Workflows

Forge now ships a `reconcile` command domain for plan-first reconciliation by execution target.

- `forge reconcile local <file-or-dir>`
- `forge reconcile remote <file-or-dir>`
- `forge reconcile local --apply <file-or-dir>`
- `forge reconcile remote --apply --strict <file-or-dir>`

Both commands build and print a plan first. They default to a dry plan and require `--apply` before mutating live state. `--strict` fails when the manifest tree contains kinds that are incompatible with the selected target instead of skipping them.

`forge reconcile remote` currently manages the staged remote kinds directly inside Forge while keeping the package layout ready for a later carve-out back into `anvil`:

- `GitHubRepository` reads the target owner from `spec.owner` (a user or organization the authenticated token can manage). Forge resolves the GitHub token in this order: `GITHUB_TOKEN`, `GH_TOKEN`, then `gh auth token` — so an already authenticated `gh` CLI is enough for day-to-day use, while CI can keep setting `GITHUB_TOKEN` explicitly. `forge manifest generate github-repo` prompts for the owner and defaults to the current GitHub login when any of those sources is available.
- `HCPTerraformWorkspace` uses `TF_TOKEN_app_terraform_io` or `TFE_TOKEN`.
- `AWSIAMProvisioner` uses the ambient AWS CLI session and expects the shared OIDC providers from `forge init aws-oidc` to exist first.

`forge reconcile local` remains the home for workstation-local kinds such as `LaunchAgent`, which do not belong in `anvil`.

## CI/CD

Forge publishes CLI artifacts through GitHub Actions workflows under `.github/workflows/`.

- `build.yaml` runs `go test ./...` for pull requests and branch pushes.
- Pushes to `main` also build Forge binaries for Linux and macOS on both `amd64` and `arm64`, then upload them as workflow artifacts.
- `publish.yaml` runs on version tags matching `v*`, rebuilds the release matrix with the tag injected into `forge --version`, and publishes `.tar.gz` archives plus `SHA256SUMS.txt` to GitHub Releases.

## Updating Forge

Forge can update itself from the public GitHub Releases feed:

- `forge update` installs the latest compatible release when a newer version is available.
- `forge update --check` reports the current version and whether a newer compatible release exists without modifying the installed binary.
- `forge update --version v0.1.0` installs a specific released version when its matching platform asset exists.

The update command verifies the downloaded archive against the published `SHA256SUMS.txt` file before replacement. The installed binary must also live in a writable location, since Forge replaces the current executable in place.

## Architecture

The repository layout, the `internal/` vs `pkg/` split, and the cobra command registration pattern that new domains follow are documented in [ARCHITECTURE.md](ARCHITECTURE.md). Strategic and architectural decisions for Forge are tracked as ADRs under [docs/adr](docs/adr/README.md).

## AI-Assisted Development

AI agents may be used in this repository for coding assistance, drafting, and documentation generation.

They are used to accelerate implementation and communication, not as a substitute for engineering judgment. Code and documentation kept in this repository are expected to be reviewed and understood by the repository author.
