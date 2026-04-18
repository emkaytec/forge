## Why

Forge can now publish versioned multi-architecture binaries, but operators still have to manually discover, download, and replace the executable to move between releases. A built-in `forge update` command turns that new release path into a first-class operator workflow and keeps the update experience inside the product shell Forge is meant to provide.

## What Changes

- Add a root-level `forge update` command that downloads and installs the latest compatible Forge release from GitHub Releases.
- Add `forge update --check` so operators can see whether a newer version is available without modifying the current executable.
- Add `forge update --version <tag>` so operators can pin installation to a specific released version.
- Detect the current operating system and architecture and resolve the matching published release asset automatically.
- Verify the downloaded binary archive against the published `SHA256SUMS.txt` file before replacing the current executable.
- Fail clearly when the current executable location is not writable or when the operator platform is unsupported.
- Keep the implementation scoped to public Forge release assets and explicit CLI behavior; do not introduce a generic updater framework or sibling-repo dependencies.

## Capabilities

### New Capabilities

- `self-update`: Operator-facing CLI behavior for checking, downloading, verifying, and installing Forge release binaries.

### Modified Capabilities

- None.

## Impact

- Affected code: new update command wiring under `internal/cli`, release lookup/download logic, checksum verification, executable replacement helpers, and tests.
- Affected docs: README/help output describing how the update command works and when it can replace the current binary.
- External systems: public GitHub Releases become a runtime dependency for update checks and binary downloads.
- Dependencies: the preferred path is standard library networking, archive handling, hashing, and file replacement rather than a dedicated updater dependency.
