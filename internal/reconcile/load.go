package reconcile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/emkaytec/forge/pkg/schema"
)

// forgeManifestDir is the hidden container that wraps per-application
// manifest directories written by `forge manifest generate` and
// `forge manifest compose` (e.g. .forge/<application>/github-repo.yaml).
// The walker descends into it even though it starts with a dot.
const forgeManifestDir = ".forge"

// DiscoverManifests returns the sorted list of manifest file paths
// rooted at path. path may be a single manifest file or a directory;
// directories are walked recursively for .yaml / .yml files so each
// application subdirectory is picked up automatically.
func DiscoverManifests(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if !IsManifestFile(path) {
			return nil, fmt.Errorf("%s is not a .yaml or .yml file", path)
		}

		return []string{path}, nil
	}

	var paths []string
	walkErr := filepath.WalkDir(path, func(entryPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden directories (e.g. .git, .terraform) so the
			// walker does not traverse version-control or tooling trees.
			// The .forge container is the exception: it's where
			// `forge manifest generate` writes manifests, so the walker
			// has to descend into it to find them.
			if entryPath != path && strings.HasPrefix(d.Name(), ".") && d.Name() != forgeManifestDir {
				return fs.SkipDir
			}
			return nil
		}
		if IsManifestFile(entryPath) {
			paths = append(paths, entryPath)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Strings(paths)

	if len(paths) == 0 {
		return nil, fmt.Errorf("%s does not contain any .yaml or .yml manifest files", path)
	}

	return paths, nil
}

// IsManifestFile reports whether path has a manifest file extension.
func IsManifestFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

// loadManifest reads and decodes a single manifest file. Returned
// errors wrap schema.DecodeManifest errors so callers can still use
// errors.As to distinguish validation vs. kind / version errors.
func loadManifest(path string) (*schema.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return schema.DecodeManifest(data)
}
