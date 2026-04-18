# Design: build-cli-shell

## Context

Forge today is a ~30-line dispatcher. MK-3 asks for a visually polished shell as the foundation for every future subcommand. The choices below are captured because they commit Forge to a dependency stack, a package layout, and a version-injection mechanism that downstream changes will inherit.

## Stack Choices

### Command routing: `spf13/cobra`

- Industry-standard for Go CLIs; provides subcommand routing, help generation, persistent flags, and unknown-command suggestions out of the box.
- Alternatives considered:
  - **Hand-rolled dispatcher (status quo).** Rejected — MK-5/MK-8/MK-11/MK-12 each add nested subcommands, which would push us toward reinventing cobra.
  - **`urfave/cli`.** Rejected — smaller ecosystem fit for the charm libraries and less idiomatic with cobra's `GroupID`-based help grouping that MK-3 explicitly references.
- Trade-off: adds a dependency tree, but the payoff across the full command roadmap outweighs the cost.

### Styling: `charmbracelet/lipgloss` + `muesli/termenv`

- `lipgloss` provides declarative styles (color, padding, border, alignment) with a consistent API.
- `termenv` detects the terminal's color profile and honors `NO_COLOR`; lipgloss uses it internally, but we also call it directly when we need to force-downgrade profiles in tests.
- All styles live in `internal/ui`. Call sites receive renderers, not raw styles, so the palette can evolve in one place.

### Async feedback: `charmbracelet/bubbletea` + `bubbles/spinner`

- Provides a reusable spinner primitive. We expose a thin wrapper (`ui.Spinner`) so command code does not take a direct bubbletea dependency.
- No interactive prompts, forms, or multi-model TUIs in this change — scope is a spinner primitive and the demo command that exercises it.

## Brand Banner

A styled `FORGE` wordmark with a flame motif is rendered on the welcome screen and available as `forge demo banner`. It's defined once in `internal/ui/banner.go` and composed from two layers:

1. **Wordmark** — block-letter `FORGE` rendered in the palette's primary color.
2. **Flame** — a small stylized flame glyph placed to the left of the wordmark, rendered with a warning→error gradient (via multiple `lipgloss.Style` passes on consecutive rune groups) so the ember reads as warm even on a 16-color profile.

Reference sketch (final glyph art finalized during implementation; this is the silhouette to match):

```
     (          ███████╗ ██████╗ ██████╗  ██████╗ ███████╗
    ) )         ██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝
   ( ( (        █████╗  ██║   ██║██████╔╝██║  ███╗█████╗
    \ | /       ██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝
     \|/        ██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗
    \_|_/       ╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
```

Rules:

- The banner function accepts an `io.Writer` and the resolved termenv profile so the same helper handles color and ASCII-only paths.
- When color is disabled (`NO_COLOR`, non-color terminal, or `--no-banner` if we add it later — not in this change), the banner still renders but without ANSI codes, preserving the silhouette.
- The banner MUST NOT be written to stderr and MUST NOT appear in explicit help output (`forge --help`, `forge help`, subcommand help, or unknown-command failure help); it is a welcome-screen and demo affordance only.
- Width budget: banner total width ≤ 60 columns so it survives an 80-column terminal with comfortable left padding.

## Demo Command Group

A top-level `demo` subcommand houses commands whose purpose is to exercise UI primitives:

- `forge demo spinner` — spins for a short bounded duration (≈2s by default so an operator can actually see it, adjustable via a hidden duration override for tests) then prints a styled success line using the `ui.Success` writer.
- `forge demo banner` — prints the brand banner once and returns.

The `demo` group uses cobra's `GroupID` so help output visually separates it from future operator command groups. These commands have no side effects beyond stdout writes and are safe to run offline.

## Package Layout

```
cmd/forge/main.go              # entrypoint, injects version, streams, args
internal/cli/
  root.go                      # cobra root command, persistent flags, version wiring
  demo.go                      # demo command group (banner, spinner)
  run.go                       # Run(args, stdout, stderr, version) — preserves existing signature shape
  run_test.go                  # behavior tests (help, version, unknown command, NO_COLOR, demo)
internal/ui/
  palette.go                   # lipgloss.Color definitions (primary, success, warning, error, muted)
  styles.go                    # exported lipgloss.Style values built from the palette
  icons.go                     # ✓ ✗ ⚠ and helpers
  writer.go                    # Success/Warn/Error writers that accept io.Writer
  profile.go                   # termenv profile detection + NO_COLOR handling, test overrides
  spinner.go                   # thin bubbletea spinner wrapper
  banner.go                    # FORGE brand banner (wordmark + flame)
```

- `internal/ui` has no dependency on `internal/cli`; the CLI layer imports UI.
- Tests in `internal/ui` use `termenv.Ascii` to force-disable color for deterministic output.

## Version Injection

- Version comes from a package-level `var version = "dev"` in `cmd/forge/main.go`, overridable via linker flags: `-ldflags "-X main.version=<value>"`.
- `main.go` passes the value into `cli.Run` so tests can exercise the flag without relying on build-time injection.
- `--version` and `-v` both print the same string with a trailing newline to stdout and return nil; they short-circuit before any subcommand dispatch.

## Verbosity Flags

- `--verbose` and `--debug` are persistent boolean flags on the root command.
- In this change they are stored on a shared `cli.Options` struct but do not yet gate output — consumers arrive in later changes. Defining them now keeps the flag surface stable for downstream work.

## Unknown Command Handling

- Cobra's `SuggestionsMinimumDistance` defaults to 2; we keep the default. When unknown, cobra writes its suggestion-bearing error; we wrap it so the returned error still includes the unknown command value (preserving the existing bootstrap contract).
- Help is still written to **stderr** on unknown-command failure; success paths write to **stdout**.

## Testability

- `cli.Run(args, stdout, stderr, version)` remains pure-function-shaped so tests can pass buffers and assert on bytes.
- UI tests set `TERMENV_PROFILE=ascii` (or call the profile override) so golden strings do not embed ANSI escape codes.
- A non-empty `NO_COLOR` value is asserted end-to-end in one test so we catch regressions in the termenv wiring.

## Rejected Alternatives

- **Splitting UI into a separate `cli-ui` capability spec.** Rejected — the UI primitives exist only to serve the CLI shell; a single capability keeps the behavioral contract cohesive.
- **Placing `spinner` and `banner` as top-level commands.** Rejected — grouping them under `demo` keeps the top-level command surface focused on operator workflows and makes it obvious these commands exist to exercise the UI layer.
- **Rendering the banner as a pre-baked string constant.** Rejected — a builder that takes the termenv profile and composes wordmark + flame layers at render time keeps color-disabled output clean and lets the palette evolve without rewriting embedded escape codes.
