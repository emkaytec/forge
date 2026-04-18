# Forge CLI Bootstrap Specification

## Purpose

Define the current bootstrap behavior of the public Forge CLI so future command work starts from an explicit, reviewable baseline.

## Command Surface

Forge currently exposes a lightweight bootstrap command surface:

- Running `forge` with no arguments shows help output.
- Running `forge help`, `forge -h`, or `forge --help` shows help output.
- Unknown commands are rejected with a clear error after help text is written to standard error.

## Inputs

- Positional command arguments supplied on the command line.
- Standard output and standard error streams supplied by the caller.

## Outputs

- Help text written to standard output for normal help flows.
- Help text written to standard error for unknown-command failures.
- A non-nil error for unknown commands so the process can exit non-zero.

## Configuration

- The bootstrap CLI does not require manifests, environment-specific configuration, secrets, or credentials.
- Project-level OpenSpec defaults live in `openspec/config.yaml`.
## Requirements
### Requirement: Help-first bootstrap surface

The Forge CLI SHALL provide a help-first shell experience that renders styled, scannable output using the shared `internal/ui` palette for every default help flow.

#### Scenario: No arguments

- GIVEN the Forge CLI is invoked with no positional arguments
- WHEN command dispatch begins
- THEN the CLI writes the brand banner followed by a styled welcome/help screen to standard output
- AND the output uses the shared palette (bold command names, muted descriptions)
- AND the CLI returns no error

#### Scenario: Explicit help

- GIVEN the Forge CLI is invoked with `help`, `-h`, or `--help`
- WHEN command dispatch begins
- THEN the CLI writes the styled help output to standard output
- AND the output does not include the brand banner
- AND the CLI returns no error

#### Scenario: Grouped help output

- GIVEN the Forge CLI renders its help output
- WHEN multiple command groups are registered
- THEN commands are visually grouped by topic using consistent section headings

### Requirement: Unknown commands fail clearly

The Forge CLI SHALL reject unknown commands with operator-facing feedback that includes a suggested closest match when one is available.

#### Scenario: Unsupported command with no close match

- GIVEN the Forge CLI is invoked with an unsupported command that has no near-match
- WHEN command dispatch begins
- THEN the CLI writes the styled help output to standard error
- AND the output does not include the brand banner
- AND the CLI returns an error whose message includes the unknown command value

#### Scenario: Unsupported command with a close match

- GIVEN the Forge CLI is invoked with a misspelled command that has a close match among registered commands
- WHEN command dispatch begins
- THEN the CLI writes a suggestion line naming the closest command to standard error
- AND the help output does not include the brand banner
- AND the CLI returns an error whose message includes the unknown command value

### Requirement: Public-safe product description

The Forge CLI SHALL describe the product in sanitized public terms in all styled output.

#### Scenario: Help copy

- GIVEN an operator reads the help output
- WHEN the help text describes Forge
- THEN it refers to Forge as an umbrella CLI for imperative engineering automation
- AND it does not include real organization-specific operational data

### Requirement: No bootstrap-time external configuration

The Forge CLI SHALL remain runnable without external configuration during bootstrap.

#### Scenario: Clean local checkout

- GIVEN a contributor has a clean local checkout of the public repository
- WHEN they run the bootstrap help flow
- THEN the CLI does not require manifests, credentials, secrets, or environment-specific configuration

### Requirement: Version reporting

The Forge CLI SHALL expose a root-level version flag that prints a version string supplied at build time.

#### Scenario: Long form

- GIVEN the Forge CLI is invoked with `--version`
- WHEN command dispatch begins
- THEN the CLI writes the injected version string followed by a newline to standard output
- AND no subcommand executes
- AND the CLI returns no error

#### Scenario: Short form

- GIVEN the Forge CLI is invoked with `-v`
- WHEN command dispatch begins
- THEN the CLI writes the injected version string followed by a newline to standard output
- AND the CLI returns no error

#### Scenario: Default version string

- GIVEN the Forge CLI is built without a version override
- WHEN the operator runs `forge --version`
- THEN the CLI prints a non-empty placeholder (e.g. `dev`) rather than an empty line

### Requirement: Global verbosity flags

The Forge CLI SHALL expose persistent `--verbose` and `--debug` boolean flags on the root command so future subcommands can read a shared verbosity level.

#### Scenario: Flags are advertised in help

- GIVEN an operator runs `forge --help`
- WHEN the CLI renders flag documentation
- THEN both `--verbose` and `--debug` appear as persistent flags in the help output

#### Scenario: Flags do not change bootstrap behavior

- GIVEN the Forge CLI is invoked with `--verbose` or `--debug` during the bootstrap phase
- WHEN no subcommand consumes the flags
- THEN the CLI renders normal help output and returns no error

### Requirement: Styled output primitives

The Forge CLI SHALL expose shared success, warning, and error output primitives that render a distinct icon and color for each severity.

#### Scenario: Success output

- GIVEN a command calls the shared success writer with a message
- WHEN the output is rendered to a color-capable writer
- THEN the output begins with the success icon `✓`
- AND the message is styled with the palette's success color

#### Scenario: Warning output

- GIVEN a command calls the shared warning writer with a message
- WHEN the output is rendered to a color-capable writer
- THEN the output begins with the warning icon `⚠`
- AND the message is styled with the palette's warning color

#### Scenario: Error output

- GIVEN a command calls the shared error writer with a message
- WHEN the output is rendered to a color-capable writer
- THEN the output begins with the error icon `✗`
- AND the message is styled with the palette's error color

### Requirement: Color profile respects environment

The Forge CLI SHALL disable color output when the terminal does not support it or when the operator has opted out.

#### Scenario: NO_COLOR opt-out

- GIVEN the environment variable `NO_COLOR` is set to any non-empty value
- WHEN the Forge CLI renders any styled output
- THEN the rendered bytes contain no ANSI escape sequences

#### Scenario: Non-color terminal

- GIVEN the active terminal profile does not advertise color support
- WHEN the Forge CLI renders any styled output
- THEN the rendered bytes contain no ANSI escape sequences

### Requirement: Brand banner

The Forge CLI SHALL render a styled `FORGE` brand banner — a block-letter wordmark paired with a flame motif — on the welcome screen and via a dedicated demo subcommand.

#### Scenario: Banner on welcome screen

- GIVEN the Forge CLI is invoked with no positional arguments
- WHEN the CLI renders the welcome output
- THEN the output begins with the brand banner
- AND the banner contains the `FORGE` wordmark silhouette
- AND the banner is no wider than 60 columns

#### Scenario: Banner color styling

- GIVEN the brand banner is rendered to a color-capable writer
- WHEN the bytes are inspected
- THEN the wordmark uses the palette's primary color
- AND the flame motif uses the palette's warning and error colors

#### Scenario: Banner without color

- GIVEN `NO_COLOR` is set or the terminal profile advertises no color support
- WHEN the brand banner is rendered
- THEN the bytes contain no ANSI escape sequences
- AND the `FORGE` wordmark silhouette is still present

### Requirement: Demo command group

The Forge CLI SHALL expose a `demo` command group that exercises the shared UI primitives without producing side effects beyond terminal output.

#### Scenario: Demo group appears in help

- GIVEN an operator runs `forge --help`
- WHEN the CLI renders its grouped help output
- THEN a `demo` group is listed with at least `banner` and `spinner` subcommands

#### Scenario: Demo banner

- GIVEN the Forge CLI is invoked with `demo banner`
- WHEN command dispatch begins
- THEN the CLI writes the brand banner once to standard output
- AND the CLI returns no error

#### Scenario: Demo spinner

- GIVEN the Forge CLI is invoked with `demo spinner`
- WHEN command dispatch begins
- THEN the CLI runs the shared spinner primitive for a bounded duration
- AND the CLI finishes by writing a styled success line using the `✓` icon
- AND the CLI returns no error

### Requirement: Async operation feedback primitive

The Forge CLI SHALL provide a reusable spinner primitive in the shared UI package so future long-running commands can surface progress consistently.

#### Scenario: Spinner lifecycle

- GIVEN a caller constructs a spinner from the shared UI package
- WHEN the caller starts the spinner and later stops it
- THEN the spinner runs without leaking goroutines and returns control to the caller

#### Scenario: Spinner on non-TTY writer

- GIVEN the spinner is run against a non-TTY writer
- WHEN the spinner starts and stops
- THEN it does not panic and produces no ANSI escape sequences in the captured bytes

