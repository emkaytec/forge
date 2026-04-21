package reconcile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
)

func TestDiscoverManifestsRecursesIntoSubdirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Mimic a "one application per subdirectory" layout the CI-style
	// `forge reconcile` invocation needs to support.
	appOne := filepath.Join(root, "app-one")
	appTwo := filepath.Join(root, "app-two", "nested")
	for _, dir := range []string{appOne, appTwo} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	writes := map[string]string{
		filepath.Join(appOne, "github-repo.yaml"):      "placeholder\n",
		filepath.Join(appOne, "hcp-tf-workspace.yml"):  "placeholder\n",
		filepath.Join(appTwo, "launch-agent.yaml"):     "placeholder\n",
		filepath.Join(root, "README.md"):               "ignored\n",
		filepath.Join(appOne, "notes.txt"):             "ignored\n",
	}
	for path, body := range writes {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}

	paths, err := reconcile.DiscoverManifests(root)
	if err != nil {
		t.Fatalf("DiscoverManifests() error = %v", err)
	}

	want := []string{
		filepath.Join(appOne, "github-repo.yaml"),
		filepath.Join(appOne, "hcp-tf-workspace.yml"),
		filepath.Join(appTwo, "launch-agent.yaml"),
	}

	if len(paths) != len(want) {
		t.Fatalf("DiscoverManifests() returned %d paths, want %d: %v", len(paths), len(want), paths)
	}
	for _, expected := range want {
		found := false
		for _, got := range paths {
			if got == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("DiscoverManifests() missing %q in %v", expected, paths)
		}
	}
}

func TestDiscoverManifestsSkipsHiddenDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	hidden := filepath.Join(root, ".git")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", hidden, err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "HEAD.yaml"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(HEAD.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "real.yaml"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(real.yaml) error = %v", err)
	}

	paths, err := reconcile.DiscoverManifests(root)
	if err != nil {
		t.Fatalf("DiscoverManifests() error = %v", err)
	}
	if len(paths) != 1 || !strings.HasSuffix(paths[0], "real.yaml") {
		t.Fatalf("DiscoverManifests() returned %v, want only real.yaml", paths)
	}
}
