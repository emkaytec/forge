package manifest

import "testing"

func TestCommandRegistersManifestSubcommands(t *testing.T) {
	t.Parallel()

	cmd := Command()

	if cmd.Use != "manifest" {
		t.Fatalf("Use = %q, want manifest", cmd.Use)
	}

	tests := []string{"generate", "validate"}
	for _, subcommandName := range tests {
		subcommand, _, err := cmd.Find([]string{subcommandName})
		if err != nil {
			t.Fatalf("Find(%s) error = %v", subcommandName, err)
		}

		if subcommand == nil {
			t.Fatalf("%s subcommand was not registered", subcommandName)
		}

		if subcommand.Name() != subcommandName {
			t.Fatalf("%s Name = %q, want %q", subcommandName, subcommand.Name(), subcommandName)
		}
	}

	if subcommand, _, err := cmd.Find([]string{"compose"}); err == nil && subcommand.Name() == "compose" {
		t.Fatal("compose subcommand should not be registered")
	}
}
