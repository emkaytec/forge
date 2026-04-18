# self-update Specification

## Purpose
TBD - created by archiving change add-self-update-command. Update Purpose after archive.
## Requirements
### Requirement: Forge exposes a self-update command

The Forge CLI SHALL expose a root-level `update` command for checking and installing released Forge binaries.

#### Scenario: Update command appears in help

- **WHEN** an operator runs `forge --help`
- **THEN** the help output lists an `update` command with an operator-facing description

#### Scenario: Check-only mode

- **WHEN** an operator runs `forge update --check`
- **THEN** Forge reports the current version and whether a newer compatible release is available
- **AND** Forge does not modify the installed binary

#### Scenario: Install specific version

- **WHEN** an operator runs `forge update --version v0.1.2`
- **THEN** Forge resolves release assets for tag `v0.1.2`
- **AND** Forge attempts to install that requested version instead of the latest release

### Requirement: Forge resolves the correct release asset for the current platform

The Forge CLI SHALL detect the current operating system and architecture and select the matching published release asset.

#### Scenario: Supported platform

- **WHEN** Forge runs on a supported platform such as `darwin/arm64`
- **THEN** it resolves the matching asset name for that platform from the target release

#### Scenario: Unsupported platform

- **WHEN** Forge runs on a platform that is not in the published release matrix
- **THEN** the update command returns a clear unsupported-platform error
- **AND** Forge does not modify the installed binary

### Requirement: Forge verifies downloaded release assets before replacement

The Forge CLI SHALL verify the downloaded archive against the published `SHA256SUMS.txt` file before replacing the current executable.

#### Scenario: Checksum matches

- **WHEN** Forge downloads the target archive and its checksum entry matches the published `SHA256SUMS.txt` file
- **THEN** Forge continues to executable replacement

#### Scenario: Checksum mismatch

- **WHEN** Forge downloads the target archive and the computed checksum does not match the published checksum entry
- **THEN** Forge returns a checksum-verification error
- **AND** Forge does not replace the current executable

### Requirement: Forge replaces the current executable safely

The Forge CLI SHALL replace the current executable only after the target archive has been downloaded, extracted, and verified successfully.

#### Scenario: Successful install

- **WHEN** a newer compatible release is available and the executable path is writable
- **THEN** Forge replaces the current executable with the verified target binary
- **AND** Forge reports the installed version to the operator

#### Scenario: Non-writable executable path

- **WHEN** the current executable path or destination directory is not writable
- **THEN** Forge returns a clear permission-related error
- **AND** Forge leaves the existing executable unchanged

### Requirement: Forge handles no-op update cases clearly

The Forge CLI SHALL tell the operator when no install action is needed.

#### Scenario: Already current on latest check

- **WHEN** an operator runs `forge update` or `forge update --check` and the current version already matches the latest compatible release
- **THEN** Forge reports that the installed version is already current
- **AND** Forge does not replace the executable

