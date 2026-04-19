package manifest

import "testing"

func TestCommandRegistersGenerateSubcommand(t *testing.T) {
	t.Parallel()

	cmd := Command()

	if cmd.Use != "manifest" {
		t.Fatalf("Use = %q, want manifest", cmd.Use)
	}

	if cmd.GroupID != GroupID {
		t.Fatalf("GroupID = %q, want %q", cmd.GroupID, GroupID)
	}

	subcommand, _, err := cmd.Find([]string{"generate"})
	if err != nil {
		t.Fatalf("Find(generate) error = %v", err)
	}

	if subcommand == nil {
		t.Fatal("generate subcommand was not registered")
	}

	if subcommand.Use != "generate" {
		t.Fatalf("generate Use = %q, want generate", subcommand.Use)
	}
}
