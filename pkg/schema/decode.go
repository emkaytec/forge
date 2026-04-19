package schema

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

type rawManifest struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       Kind      `yaml:"kind"`
	Metadata   Metadata  `yaml:"metadata"`
	Spec       yaml.Node `yaml:"spec"`
}

// DecodeManifest unmarshals YAML into the supported typed manifest envelope.
func DecodeManifest(data []byte) (*Manifest, error) {
	var raw rawManifest
	if err := decodeStrictYAML(data, &raw); err != nil {
		return nil, fmt.Errorf("schema: decode manifest envelope: %w", err)
	}

	if err := ValidateAPIVersion(raw.APIVersion); err != nil {
		return nil, err
	}

	if err := ValidateKind(raw.Kind); err != nil {
		return nil, err
	}

	spec, err := decodeSpec(raw.Kind, &raw.Spec)
	if err != nil {
		return nil, err
	}

	manifest := &Manifest{
		APIVersion: raw.APIVersion,
		Kind:       raw.Kind,
		Metadata:   raw.Metadata,
		Spec:       spec,
	}

	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	return manifest, nil
}

func decodeSpec(kind Kind, node *yaml.Node) (any, error) {
	switch kind {
	case KindGitHubRepo:
		var spec GitHubRepoSpec
		if err := decodeNodeStrict(node, &spec); err != nil {
			return nil, fmt.Errorf("schema: decode %q spec: %w", kind, err)
		}
		return &spec, nil
	case KindHCPTFWorkspace:
		var spec HCPTFWorkspaceSpec
		if err := decodeNodeStrict(node, &spec); err != nil {
			return nil, fmt.Errorf("schema: decode %q spec: %w", kind, err)
		}
		return &spec, nil
	case KindAWSIAMProvisioner:
		var spec AWSIAMProvisionerSpec
		if err := decodeNodeStrict(node, &spec); err != nil {
			return nil, fmt.Errorf("schema: decode %q spec: %w", kind, err)
		}
		return &spec, nil
	case KindLaunchAgent:
		var spec LaunchAgentSpec
		if err := decodeNodeStrict(node, &spec); err != nil {
			return nil, fmt.Errorf("schema: decode %q spec: %w", kind, err)
		}
		return &spec, nil
	default:
		return nil, &UnsupportedKindError{Kind: string(kind)}
	}
}

func decodeStrictYAML(data []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(out)
}

func decodeNodeStrict(node *yaml.Node, out any) error {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(node); err != nil {
		return err
	}

	if err := encoder.Close(); err != nil {
		return err
	}

	return decodeStrictYAML(buf.Bytes(), out)
}
