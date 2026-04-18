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

The Forge CLI SHALL provide a help-first bootstrap experience until operator workflows are intentionally added.

#### Scenario: No arguments

- GIVEN the Forge CLI is invoked with no positional arguments
- WHEN command dispatch begins
- THEN the CLI writes help output to standard output
- AND the CLI returns no error

#### Scenario: Explicit help

- GIVEN the Forge CLI is invoked with `help`, `-h`, or `--help`
- WHEN command dispatch begins
- THEN the CLI writes help output to standard output
- AND the CLI returns no error

### Requirement: Unknown commands fail clearly

The Forge CLI SHALL reject unknown commands with explicit operator-facing feedback.

#### Scenario: Unsupported command

- GIVEN the Forge CLI is invoked with an unsupported command
- WHEN command dispatch begins
- THEN the CLI writes help output to standard error
- AND the CLI returns an error that includes the unknown command value

### Requirement: Public-safe product description

The Forge CLI SHALL describe the product in sanitized public terms.

#### Scenario: Help copy

- GIVEN an operator reads the bootstrap help output
- WHEN the help text describes Forge
- THEN it refers to Forge as an umbrella CLI for imperative engineering automation
- AND it does not include real organization-specific operational data

### Requirement: No bootstrap-time external configuration

The Forge CLI SHALL remain runnable without external configuration during bootstrap.

#### Scenario: Clean local checkout

- GIVEN a contributor has a clean local checkout of the public repository
- WHEN they run the bootstrap help flow
- THEN the CLI does not require manifests, credentials, secrets, or environment-specific configuration
