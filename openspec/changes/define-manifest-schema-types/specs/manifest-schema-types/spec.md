## ADDED Requirements

### Requirement: Forge manifests use a typed envelope

Forge SHALL represent supported manifests with a YAML envelope containing `apiVersion`, `kind`, `metadata`, and `spec`, and SHALL dispatch `spec` into a typed schema based on the declared `kind`.

#### Scenario: Supported manifest envelope
- **WHEN** a manifest declares `apiVersion: forge/v1`, a supported `kind`, `metadata.name`, and a matching `spec`
- **THEN** Forge decodes the document into the manifest envelope plus the typed schema for that kind
- **AND** the decoded manifest preserves the declared `metadata.name`

#### Scenario: Unsupported kind
- **WHEN** a manifest declares an unknown `kind`
- **THEN** Forge returns an unknown-kind schema error
- **AND** Forge does not decode the `spec` into a typed resource struct

#### Scenario: Unsupported apiVersion
- **WHEN** a manifest declares an `apiVersion` other than `forge/v1`
- **THEN** Forge returns an unsupported-version schema error

### Requirement: Forge supports the `github-repo` manifest kind

Forge SHALL support a `github-repo` manifest kind with the initial schema fields `name`, `visibility`, `description`, `topics`, `default_branch`, and `branch_protection`.

#### Scenario: Minimal GitHub repository manifest
- **WHEN** a `github-repo` manifest provides `name` and `visibility`
- **THEN** Forge accepts the manifest as schema-valid
- **AND** the manifest decodes into the typed GitHub repository schema

#### Scenario: Default branch omitted
- **WHEN** a `github-repo` manifest omits `default_branch`
- **THEN** Forge treats the schema value as `main`

#### Scenario: Invalid visibility
- **WHEN** a `github-repo` manifest sets `visibility` to a value other than `public` or `private`
- **THEN** Forge returns a schema validation error

### Requirement: Forge supports the `hcp-tf-workspace` manifest kind

Forge SHALL support an `hcp-tf-workspace` manifest kind with the initial schema fields `name`, `organization`, `project`, `vcs_repo`, `execution_mode`, and `terraform_version`.

#### Scenario: Minimal HCP Terraform workspace manifest
- **WHEN** an `hcp-tf-workspace` manifest provides `name`, `organization`, and `execution_mode`
- **THEN** Forge accepts the manifest as schema-valid
- **AND** the manifest decodes into the typed HCP Terraform workspace schema

#### Scenario: Invalid execution mode
- **WHEN** an `hcp-tf-workspace` manifest sets `execution_mode` to a value other than `remote`, `local`, or `agent`
- **THEN** Forge returns a schema validation error

### Requirement: Forge supports the `aws-iam-provisioner` manifest kind

Forge SHALL support an `aws-iam-provisioner` manifest kind that models only OIDC-backed provisioner roles with the initial schema fields `name`, `account_id`, `oidc_provider`, `oidc_subject`, and `managed_policies`.

#### Scenario: OIDC-backed provisioner manifest
- **WHEN** an `aws-iam-provisioner` manifest provides `name`, `account_id`, `oidc_provider`, and `oidc_subject`
- **THEN** Forge accepts the manifest as schema-valid
- **AND** the manifest decodes into the typed AWS IAM provisioner schema

#### Scenario: Unsupported general IAM fields
- **WHEN** an `aws-iam-provisioner` manifest includes unsupported general IAM role fields outside the declared schema
- **THEN** Forge returns a schema error instead of silently accepting the extra fields

### Requirement: Forge supports the `launch-agent` manifest kind

Forge SHALL support a `launch-agent` manifest kind with the initial schema fields `name`, `label`, `command`, `schedule`, and `run_at_load`.

#### Scenario: Calendar-based launch agent
- **WHEN** a `launch-agent` manifest sets `schedule.type` to `calendar` and provides `schedule.hour` plus `schedule.minute`
- **THEN** Forge accepts the manifest as schema-valid
- **AND** the manifest decodes into the typed launch-agent schema

#### Scenario: Interval-based launch agent
- **WHEN** a `launch-agent` manifest sets `schedule.type` to `interval` and provides `schedule.interval_seconds`
- **THEN** Forge accepts the manifest as schema-valid

#### Scenario: Incomplete schedule
- **WHEN** a `launch-agent` manifest omits the schedule fields required by the selected `schedule.type`
- **THEN** Forge returns a schema validation error

#### Scenario: Run-at-load omitted
- **WHEN** a `launch-agent` manifest omits `run_at_load`
- **THEN** Forge treats the schema value as `false`

### Requirement: Supported manifests round-trip through YAML without losing declared schema fields

Forge SHALL preserve the supported schema fields for each manifest kind when a manifest is unmarshaled into the typed schema and marshaled back to YAML.

#### Scenario: Round-trip supported manifest
- **WHEN** a supported manifest kind is unmarshaled into its typed schema and then marshaled back to YAML
- **THEN** the resulting YAML still contains the same supported envelope and schema field values
- **AND** no unsupported fields are introduced during the round-trip
