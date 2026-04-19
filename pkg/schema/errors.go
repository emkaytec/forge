package schema

import "fmt"

// UnsupportedVersionError reports an unsupported manifest apiVersion.
type UnsupportedVersionError struct {
	Version string
}

func (e *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("schema: unsupported apiVersion %q", e.Version)
}

// UnsupportedKindError reports an unsupported manifest kind.
type UnsupportedKindError struct {
	Kind string
}

func (e *UnsupportedKindError) Error() string {
	return fmt.Sprintf("schema: unsupported kind %q", e.Kind)
}

// ValidationError reports a schema validation failure for a specific field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("schema: invalid %s: %s", e.Field, e.Message)
}

// ValidateAPIVersion reports whether the manifest apiVersion is supported.
func ValidateAPIVersion(version string) error {
	if version != APIVersionV1 {
		return &UnsupportedVersionError{Version: version}
	}

	return nil
}

// ValidateKind reports whether the manifest kind is supported.
func ValidateKind(kind Kind) error {
	switch kind {
	case KindGitHubRepo, KindHCPTFWorkspace, KindAWSIAMProvisioner, KindLaunchAgent:
		return nil
	default:
		return &UnsupportedKindError{Kind: string(kind)}
	}
}

func invalidField(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}
