# Forge

Forge is a Go CLI for imperative automations across cloud infrastructure, DevOps workflows, and local development environment setup.

This repository is the public product repository for Forge. It is intended to show the product direction, implementation patterns, and documentation without exposing real production data.

## Status

This repository is an initial working scaffold.

The current bootstrap establishes the public project shape, a lightweight Go CLI entrypoint, Anvil-oriented manifest authoring and validation, and the durable guidance/docs structure the repository will build on.

Forge is intended to act as the umbrella CLI in the broader repo family. That does not erase the existing roles of sibling projects overnight:

- `forge` is the product shell for operator-facing imperative automation
- `anvil` owns the Terraform-first baseline architecture workflow that consumes Forge-authored manifests
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
go run ./cmd/forge manifest generate github-repo docs-site
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

Forge ships a `manifest` command domain for authoring and validating the Anvil Terraform YAML consumed from the root `.forge/` directory.

- `forge manifest generate github-repo <name>`
- `forge manifest validate <file-or-directory>`

`forge manifest generate github-repo` writes one `GitHubRepository` manifest to `.forge/<name>.yaml`. The generated YAML uses `apiVersion: anvil.emkaytec.dev/v1alpha1`, nests repository settings under `spec.repository`, and keeps the optional Terraform workspace fan-out in the same file.

For a standalone repository:

```bash
go run ./cmd/forge manifest generate github-repo docs-site
```

For a Terraform-backed repository, pass `--terraform` or answer yes when prompted. Forge then asks for the minimum environment and AWS account information needed by the Anvil module workflow:

```bash
go run ./cmd/forge manifest generate github-repo complete-service \
  --terraform \
  --environment admin \
  --account-id 123456789012
```

The resulting Terraform-backed manifest includes `spec.createTerraformWorkspaces: true`, `spec.environments.<environment>.aws.accountId`, and conservative workspace defaults such as `workspace.executionMode: remote`. Optional details like descriptions, topics, homepage, project name, and Terraform version can be supplied with flags without expanding the interactive prompt flow.

Generated examples use sanitized placeholder data only. See [examples/github-repo.yaml](examples/github-repo.yaml) for the standalone shape.

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
