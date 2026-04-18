# Change: build-cli-shell

## Why

The current Forge CLI is a minimal help-first dispatcher (see `forge-cli-bootstrap`). Every future operator workflow that intentionally lives under Forge — manifest, workstation, init, and similar imperative automation entrypoints — will render output through this shell, so its UX quality is load-bearing. MK-3 calls for a polished, opinionated CLI shell that provides consistent color, iconography, verbosity controls, and async feedback, so subsequent command work starts from a deliberate baseline rather than drifting into ad-hoc `fmt.Fprintln` calls.

## What Changes

- Adopt `spf13/cobra` as the command router and help generator, replacing the hand-rolled `switch` in `internal/cli/run.go`.
- Introduce an `internal/ui` package that owns the color palette, icon set (`✓`, `✗`, `⚠`), and styled renderers built on `charmbracelet/lipgloss`.
- Detect terminal color profile and honor any non-empty `NO_COLOR` value via `muesli/termenv`, wired through the `internal/ui` package so no call site touches raw ANSI.
- Add a `--version` / `-v` root flag that prints a build-time version string.
- Add persistent `--verbose` / `--debug` flags that future commands can read for log-level decisions.
- Handle unknown commands with cobra's built-in suggestion logic (`SuggestionsMinimumDistance`), preserving the existing stderr + non-zero exit contract.
- Provide a spinner primitive in `internal/ui` built on `charmbracelet/bubbletea` + `bubbles/spinner` for future long-running commands, and ship a `forge demo spinner` subcommand under a `demo` group so operators can exercise it end-to-end with a short visible delay.
- Add a styled **FORGE** brand banner (block-letter wordmark with a stylized flame motif) rendered in the palette's primary + warning colors on the welcome screen and in `forge demo banner`.
- Keep the Forge boundary intact: no manifests, secrets, or reconciliation logic — this change is presentation-only.

## Impact

- Affected specs: `forge-cli-bootstrap` (modified help/unknown-command requirements; new requirements for version flag, global verbosity, styled output, color-profile handling, and async feedback primitives).
- Affected code: `cmd/forge/main.go`, `internal/cli/run.go`, `internal/cli/run_test.go`, new `internal/ui/` package, new tests.
- New dependencies in `go.mod`: `github.com/spf13/cobra`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles`, `github.com/muesli/termenv`.
- Build output remains a single static Go binary (no cgo, no runtime assets).
- Public-repo posture unchanged: no real operational data introduced.
- Unblocks downstream issues MK-5, MK-8, MK-11, MK-12 which assume a shared styled shell.

## Out of Scope

- New operator subcommands (manifest, workstation, init). Those arrive in follow-on changes and will consume the primitives introduced here.
- Telemetry, logging backends, or structured log formats beyond what `--verbose`/`--debug` flags expose as boolean state.
- Interactive TUI flows beyond a reusable spinner primitive and the demo command that exercises it.
