## Context

Forge now has a tag-driven release workflow that publishes versioned platform archives and a shared checksum file, but operators still need a separate curl or `gh release download` flow to install updates. That is workable for bootstrap, but it leaves a core operator workflow outside the CLI even though Forge is meant to be the umbrella shell for imperative engineering tasks.

This change introduces a self-update command that consumes the public GitHub Release artifacts Forge already produces. The implementation needs to stay public-safe, avoid sibling-repo coupling, verify what it downloads, and fail clearly when the installed binary cannot be replaced in place.

## Goals / Non-Goals

**Goals:**

- Add a root-level `forge update` command that can check for and install newer Forge binaries from public GitHub Releases.
- Support both `forge update --check` and `forge update --version <tag>` as explicit operator workflows.
- Detect the current platform, select the matching release asset, verify it against `SHA256SUMS.txt`, and replace the current executable safely.
- Keep the implementation lightweight and easy to debug, preferring Go standard library packages over an updater framework dependency.
- Pair the command with tests and public documentation so operators understand the behavior and failure modes.

**Non-Goals:**

- Supporting package-manager updates such as Homebrew, apt, or asdf.
- Supporting Windows in this first pass.
- Adding background auto-update behavior, scheduled checks, or telemetry.
- Introducing a generic plugin or package installer abstraction.
- Moving release or update logic into `anvil` or `alloy`.

## Decisions

### Use public GitHub Release metadata as the update source

Query the public GitHub Releases API for the latest release in the default flow and resolve a specific tag when `--version <tag>` is provided.

Why this over scraping HTML or introducing a custom manifest:

- Forge already publishes public release assets there.
- The API is explicit, stable, and easy to exercise in tests.
- It keeps the command aligned with the release workflow contract that already exists.

Alternative considered:

- Use GitHub release download URLs without an API lookup. Rejected because `--check` needs structured latest-version metadata and clearer error handling for missing releases.

### Keep the updater in standard-library code

Use `net/http`, `encoding/json`, `archive/tar`, `compress/gzip`, `crypto/sha256`, and `os`/`filepath` helpers rather than a dedicated self-update dependency.

Why this over a library:

- The behavior is narrow and explicit.
- The repo guidance prefers standard library solutions unless a dependency is clearly justified.
- A small updater path is easier to reason about for a public portfolio-oriented repository.

Alternative considered:

- Adopt a self-update helper library. Rejected because it hides important behavior and adds a dependency for a small, inspectable workflow.

### Model the command as explicit check/install modes

Expose:

- `forge update` to install the latest compatible version when newer than the current version
- `forge update --check` to report current versus target version without modifying the binary
- `forge update --version <tag>` to install a specific released version

Why this surface:

- It covers the practical operator workflows you called out.
- It keeps behavior explicit rather than multiplexing install and check logic through positional arguments.
- It maps cleanly to test cases and help output.

Alternative considered:

- Separate `forge update` and `forge upgrade` commands. Rejected because a single verb with flags is smaller and clearer at this stage.

### Replace the executable with a temp-file plus atomic rename

Download and verify the target archive into a temporary directory, extract the `forge` binary, then replace the current executable path with a rename after confirming the destination directory is writable.

Why this approach:

- It minimizes the time spent with a partially written destination binary.
- It provides a clear point to stop before mutation if checksum verification fails.
- It works naturally with `os.Executable()` and a single-binary distribution model.

Alternative considered:

- Stream the downloaded bytes directly into the current executable path. Rejected because it risks leaving a corrupt binary behind on partial download or verification failure.

### Fail clearly on unsupported install layouts

The updater should explicitly error when:

- the platform cannot be mapped to a supported release asset
- the binary path cannot be determined
- the destination directory is not writable
- the requested version or matching asset does not exist

Why this matters:

- Self-update is safety-sensitive. Operators need a clear explanation before the command mutates the installed binary.
- Many local installs will land under user-writable paths, but some system-wide installs will not.

## Risks / Trade-offs

- [GitHub API or asset availability becomes a runtime dependency] -> Mitigation: surface clear network and missing-release errors and keep `--check`/install behavior explicit rather than silent.
- [Executable replacement can fail on read-only or root-owned install paths] -> Mitigation: preflight the destination path and return an operator-facing error before replacing anything.
- [Checksum verification adds more moving parts] -> Mitigation: keep the verification path small, deterministic, and test it against fixture inputs.
- [The command only supports platforms that match published assets] -> Mitigation: treat unsupported platforms as a clear first-class error and document the supported matrix.

## Migration Plan

1. Add the `update` command to the root CLI command tree and document the intended flags.
2. Implement release lookup, asset resolution, archive extraction, checksum verification, and executable replacement helpers.
3. Add tests for command behavior, unsupported platform handling, checksum validation, and version selection.
4. Update the README/help text to describe the new command and how it interacts with the published release assets.

Rollback strategy:

- Remove the command wiring if the workflow proves too risky.
- If the command surface is sound but install replacement is problematic, temporarily keep `--check` and disable binary replacement while iterating.

## Open Questions

None at this time.
