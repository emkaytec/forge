# cli-release-workflow Specification

## Purpose
TBD - created by archiving change set-up-multi-arch-cli-cicd-workflow. Update Purpose after archive.
## Requirements
### Requirement: Workflow responsibilities are separated by file

The Forge repository SHALL define branch/main build behavior and tag-driven publish behavior in separate GitHub Actions workflow files.

#### Scenario: Build workflow file

- **WHEN** a contributor inspects the repository workflows for CLI validation and `main` branch artifact builds
- **THEN** the repository contains a `.github/workflows/build.yaml` workflow for that behavior

#### Scenario: Publish workflow file

- **WHEN** a contributor inspects the repository workflows for tag-driven release publication
- **THEN** the repository contains a `.github/workflows/publish.yaml` workflow for that behavior

### Requirement: Pull requests and non-main branch pushes validate the CLI

The Forge repository SHALL run the Go test suite for pull requests and non-`main` branch pushes without producing public release assets.

#### Scenario: Pull request validation

- **WHEN** a pull request workflow run starts for the Forge repository
- **THEN** the workflow runs `go test ./...`
- **AND** the workflow does not publish a GitHub Release

#### Scenario: Non-main branch push validation

- **WHEN** a push workflow run starts for a branch other than `main`
- **THEN** the workflow runs `go test ./...`
- **AND** the workflow does not upload multi-architecture release archives for publication
- **AND** the workflow does not publish a GitHub Release

### Requirement: Main branch pushes produce build artifacts

The Forge repository SHALL build CLI artifacts on pushes to `main` after tests pass.

#### Scenario: Main branch build matrix

- **WHEN** a workflow run starts for a push to `main`
- **THEN** the workflow runs `go test ./...`
- **AND** the workflow builds Forge binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`
- **AND** the workflow uploads the built binaries as GitHub Actions artifacts

#### Scenario: Main branch does not publish a release

- **WHEN** a workflow run starts for a push to `main`
- **THEN** the workflow does not create or update a GitHub Release

### Requirement: Version tags publish multi-architecture CLI releases

The Forge repository SHALL publish CLI release assets from version tags that match the supported release matrix.

#### Scenario: Tagged release builds and publishes assets

- **WHEN** a workflow run starts for a pushed tag matching `v*`
- **THEN** the workflow runs `go test ./...`
- **AND** the workflow builds Forge release archives for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`
- **AND** each archive is published as a GitHub Release asset
- **AND** the workflow publishes a checksum file for the release assets

#### Scenario: Tagged release stays CLI-only

- **WHEN** a workflow run starts for a pushed tag matching `v*`
- **THEN** the workflow publishes CLI binary artifacts
- **AND** the workflow does not require container image build or publish steps

### Requirement: Published release binaries report the release version

The Forge repository SHALL inject the Git tag version into published release binaries at build time.

#### Scenario: Tagged binary reports matching version

- **WHEN** a Forge binary is built for a pushed tag matching `v*`
- **THEN** the build injects the tag value into the CLI version string
- **AND** an operator running the published binary with `forge --version` receives that tag value

