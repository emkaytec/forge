## Context

Forge currently has a bootstrap Go CLI and no GitHub Actions workflow for validating, packaging, or releasing binaries. Sibling repos such as `anvil` and `smyth` already prove out the general release shape, but they also include container publishing that Forge explicitly does not need for this change.

MK-19 asks for a CLI-distribution workflow with three distinct behaviors:

- test only for pull requests and non-`main` branch pushes
- test plus build artifacts on `main`
- test plus multi-arch release publication on version tags

The design needs to keep Forge’s public-repo posture intact, avoid introducing operational data, and stay easy to debug for a small Go CLI.

## Goals / Non-Goals

**Goals:**

- Add `build.yaml` and `publish.yaml` GitHub Actions workflows that together cover branch validation, `main` branch builds, and tag-driven release publication.
- Produce binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`.
- Inject a meaningful version string into release binaries so `forge --version` matches the published tag.
- Keep job boundaries and permissions explicit so operators can tell why a ref produced tests, artifacts, or a public release.
- Reuse simple Go cross-compilation rather than introducing heavyweight release tooling.

**Non-Goals:**

- Publishing containers, Homebrew formulas, package-manager metadata, or installer scripts.
- Adding Windows targets.
- Introducing secrets, manifests, or environment-specific deployment behavior.
- Moving build logic into `anvil` or `alloy`, which are outside this repository boundary.

## Decisions

### Use separate `build.yaml` and `publish.yaml` workflow files

Create two workflows under `.github/workflows/`:

- `build.yaml` for `pull_request` and branch-push validation, including the `main`-only artifact build path
- `publish.yaml` for pushed version tags matching `v*`, including release asset packaging and GitHub Release publication

Why this over one ref-sensitive workflow:

- It matches your preferred repository layout directly.
- It mirrors the conceptual split between "prove the repo builds" and "publish a public release."
- It keeps tag-only release permissions and logic out of the everyday branch-validation workflow.

Alternative considered:

- A single `cli-release.yml` workflow with job-level `if:` guards. Rejected because the desired repo shape is explicit, and the two-file split is still small and easy to debug.

### Cross-compile on Linux runners with a fixed matrix

Use `ubuntu-latest` for all build jobs and compile the four target binaries by setting `GOOS`, `GOARCH`, and `CGO_ENABLED=0`.

Why this over native target runners:

- The current Forge binary is a simple Go CLI with no cgo dependency.
- Linux runners are cheaper, faster to schedule, and simpler to maintain than mixing macOS runners into the matrix.
- The target list is small and explicit, which keeps the workflow readable.

Alternatives considered:

- Native macOS runners for Darwin targets. Rejected because cross-compilation is sufficient for the current binary shape.
- Goreleaser. Rejected because the repository does not need an extra release dependency to satisfy this workflow.

### Keep test, build, and publish as separate job phases

Define the workflows and jobs as:

- `build.yaml`
  - `test`: runs `go test ./...` for pull requests and branch pushes
  - `build-main-artifacts`: runs only on `refs/heads/main` after `test`
- `publish.yaml`
  - `test`: runs `go test ./...` for tag refs before any release work
  - `build-release-assets`: runs only for pushed tags matching `v*`
  - `publish-release`: downloads tag artifacts, generates checksums, and creates or replaces the GitHub release

Why this split:

- It matches the acceptance criteria directly.
- It keeps publish permissions isolated to the publish workflow instead of every build run.
- It makes it obvious whether a run came from branch validation or release publication.

Alternative considered:

- A single build job with step-level conditionals. Rejected because job-level separation produces cleaner logs and permission boundaries.

### Publish archived release assets and checksums

For tag builds, package each binary as `forge-<goos>-<goarch>.tar.gz`, upload them as workflow artifacts, then publish them together with a `SHA256SUMS.txt` file in the GitHub release.

Why this format:

- It matches the lightweight release pattern already used in sibling repos.
- A stable archive naming convention is easy to document and consume.
- Checksums improve operator trust without adding external services.

Alternative considered:

- Upload raw binaries directly to the release. Rejected because archives create a consistent asset shape and leave room for future README/LICENSE inclusion if needed.

### Inject version strings at build time

Pass `-ldflags "-X main.version=<value>"` to `go build`.

- On tag builds, `<value>` is `${GITHUB_REF_NAME}`.
- On `main`, `<value>` can be a non-release snapshot value such as the short commit SHA so uploaded artifacts are still identifiable.

Why this approach:

- `cmd/forge/main.go` already exposes a build-time version variable.
- It keeps runtime version reporting aligned with the asset an operator downloaded.

Alternative considered:

- Leaving all workflow-built artifacts at the default `dev` version. Rejected because release assets would not identify themselves correctly once downloaded.

## Risks / Trade-offs

- [Cross-compilation assumes a cgo-free binary] -> Mitigation: force `CGO_ENABLED=0` in workflow builds and revisit the strategy only if Forge adds native dependencies later.
- [One workflow file centralizes more logic] -> Mitigation: keep job names explicit, share as little shell logic as possible, and gate behavior with small, readable `if:` expressions.
- [Push and pull request events can both run tests for the same branch] -> Mitigation: use workflow `concurrency` to cancel superseded runs and accept the small amount of duplicated validation as a trade-off for explicit PR coverage.
- [Release publication can fail if a tag is re-run against an existing GitHub Release] -> Mitigation: delete any existing release object for the tag before creating a fresh one, matching the sibling-repo workaround that already proved useful.

## Migration Plan

1. Add `build.yaml` and `publish.yaml` under `.github/workflows/`.
2. Implement the shared build matrix, version injection, artifact upload, and tag-release publication logic across the two workflows.
3. Add or update concise public documentation describing how `main` artifacts and tagged releases are produced.
4. Validate locally where practical, then rely on GitHub Actions runs for end-to-end confirmation on branch, `main`, and tag refs.

Rollback strategy:

- Disable or remove the specific workflow file that is misbehaving.
- If only release publication is problematic, keep `build.yaml` intact while temporarily disabling `publish.yaml`.

## Open Questions

None at this time.
