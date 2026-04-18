## 1. CLI surface

- [x] 1.1 Add a root-level `update` command under `internal/cli` with `--check` and `--version` flags and help text that describes the operator-facing behavior.
- [x] 1.2 Introduce a small update options/result model so the command wiring can distinguish check-only, latest-install, and pinned-version flows cleanly.

## 2. Release lookup and installation

- [x] 2.1 Implement release lookup against the public GitHub Releases API for latest and tag-specific resolution using standard library HTTP and JSON handling.
- [x] 2.2 Implement platform-to-asset resolution for the supported Forge release matrix and return clear errors for unsupported platforms or missing assets.
- [x] 2.3 Implement archive download, `SHA256SUMS.txt` verification, and extraction of the `forge` binary from the matching `.tar.gz` asset.
- [x] 2.4 Implement safe executable replacement using `os.Executable()`, a temporary download/extract directory, and an atomic rename after writability checks.
- [x] 2.5 Add operator-facing output paths for update available, already current, checksum failure, permission failure, and requested-version-not-found cases.

## 3. Tests and docs

- [x] 3.1 Add tests for command behavior, platform resolution, checksum verification, and executable replacement failure/no-op cases.
- [x] 3.2 Update the public README and any help expectations/tests to describe `forge update`, `forge update --check`, and `forge update --version`.
- [x] 3.3 Run `go test ./...` and `openspec validate "add-self-update-command" --strict` to confirm the implementation and spec bundle stay aligned.
