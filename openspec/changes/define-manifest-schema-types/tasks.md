## 1. Package scaffolding

- [x] 1.1 Add `gopkg.in/yaml.v3` to `go.mod` / `go.sum` and create `pkg/schema/` with a package comment that marks the code as an alloy candidate
- [x] 1.2 Add shared manifest envelope types for `apiVersion`, `kind`, `metadata`, and raw `spec` decoding under `pkg/schema/`
- [x] 1.3 Add supported version and kind constants plus shared schema error helpers for unsupported kinds or versions

## 2. Typed schema implementation

- [x] 2.1 Add the `github-repo` schema type with `visibility` validation and `default_branch` defaulting to `main`
- [x] 2.2 Add the `hcp-tf-workspace` schema type with `execution_mode` validation for `remote`, `local`, and `agent`
- [x] 2.3 Add the `aws-iam-provisioner` schema type scoped to OIDC-backed provisioner roles only
- [x] 2.4 Add the `launch-agent` schema type with interval-vs-calendar schedule validation and `run_at_load` defaulting to `false`
- [x] 2.5 Wire manifest decoding so supported kinds dispatch into the correct typed schema and unknown fields are rejected

## 3. Tests

- [x] 3.1 Add envelope decode tests covering supported manifests plus unsupported `apiVersion` and `kind` errors
- [x] 3.2 Add YAML round-trip and validation tests for `github-repo` and `hcp-tf-workspace`
- [x] 3.3 Add YAML round-trip and validation tests for `aws-iam-provisioner` and `launch-agent`, including unsupported extra fields and incomplete schedules

## 4. Documentation and validation

- [x] 4.1 Add a new ADR under `docs/adr/` capturing the `pkg/schema` staging decision, the narrow initial field sets, and references to anvil ADR-0004 and ADR-0005
- [x] 4.2 Confirm `pkg/README.md` and `internal/manifest/README.md` still describe the schema boundary accurately; update them only if the new package makes the current wording stale
- [x] 4.3 Run `go test ./...`
- [x] 4.4 Run `openspec validate define-manifest-schema-types --strict`
