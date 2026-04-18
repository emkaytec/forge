## Why

Forge has a usable bootstrap CLI, but it does not yet have a release path for distributing operator-ready binaries. MK-19 closes that gap by defining GitHub Actions workflows that keep branch validation lightweight, produce build artifacts on `main`, and publish tagged multi-architecture CLI releases without turning Forge into a container-first project.

## What Changes

- Add `build.yaml` and `publish.yaml` GitHub Actions workflows for Forge CLI distribution, separating branch/main validation from tag-driven release publication.
- Run `go test ./...` for pull requests and non-`main` branch pushes so day-to-day validation stays fast and low-risk.
- Build Forge binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64` on `main` and upload them as workflow artifacts for inspection.
- Build release archives for the same target matrix on version tags and publish them as GitHub release assets, including checksums.
- Inject the tagged version into the compiled CLI binaries at build time so released artifacts report the same version the release advertises.
- Keep the change scoped to CLI distribution only; do not add container publishing, reconciliation behavior, manifests, or private operational data.

## Capabilities

### New Capabilities

- `cli-release-workflow`: GitHub Actions behavior for testing, building, archiving, and publishing Forge CLI binaries across the supported release matrix.

### Modified Capabilities

- None.

## Impact

- Affected code and config: new `.github/workflows/build.yaml` and `.github/workflows/publish.yaml` files, release build commands, and any small Makefile or README touchpoints needed to explain the distribution path.
- Affected runtime surface: released binaries gain build-time version injection consistency across supported operating systems and CPU architectures.
- External systems: GitHub Actions and GitHub Releases become the public distribution path for Forge CLI artifacts.
- Dependencies and permissions: workflow jobs require standard GitHub-hosted runner support for Go builds, artifact upload/download, and `contents: write` on tag-driven release publication.
