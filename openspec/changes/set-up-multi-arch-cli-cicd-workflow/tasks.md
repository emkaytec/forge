## 1. Workflow scaffold

- [x] 1.1 Add `.github/workflows/build.yaml` with `pull_request` and branch-push triggers plus concurrency and baseline permissions.
- [x] 1.2 Add `.github/workflows/publish.yaml` with the `v*` tag trigger and the release-specific permissions needed for publication.
- [x] 1.3 Add a `test` job to both workflows that checks out the repo, sets up Go from `go.mod`, and runs `go test ./...` for the refs each workflow owns.

## 2. Build and release behavior

- [ ] 2.1 Add a `main`-only build job that cross-compiles Forge binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64` and uploads them as workflow artifacts.
- [ ] 2.2 Add a tag-only release-asset build job in `publish.yaml` that cross-compiles the same matrix with build-time version injection and packages each target as `forge-<goos>-<goarch>.tar.gz`.
- [ ] 2.3 Add a tag-only publish job in `publish.yaml` that downloads the archived assets, generates `SHA256SUMS.txt`, replaces any existing release for the tag, and publishes the GitHub release assets.

## 3. Documentation and verification

- [ ] 3.1 Update the public repo docs with a concise description of branch validation, `main` build artifacts, and tag-driven CLI releases.
- [ ] 3.2 Validate the change by reviewing the workflow ref guards and target matrix, then run `go test ./...` so the repo still passes locally after the workflow and doc edits.
