package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePlatform(t *testing.T) {
	t.Parallel()

	if _, err := resolvePlatform("darwin", "arm64"); err != nil {
		t.Fatalf("expected supported platform, got %v", err)
	}

	if _, err := resolvePlatform("windows", "amd64"); err == nil {
		t.Fatal("expected unsupported platform error")
	}
}

func TestVerifyChecksumMismatch(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "forge-darwin-arm64.tar.gz")
	checksumPath := filepath.Join(tempDir, checksumAssetName)

	if err := os.WriteFile(archivePath, []byte("not-a-real-archive"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := os.WriteFile(checksumPath, []byte("deadbeef forge-darwin-arm64.tar.gz\n"), 0o644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	err := verifyChecksum(archivePath, checksumPath, "forge-darwin-arm64.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "checksum verification failed") {
		t.Fatalf("expected checksum verification error, got %v", err)
	}
}

func TestUpdaterRunCheckOnly(t *testing.T) {
	t.Parallel()

	server, executablePath, originalContents := newTestUpdateServer(t, "v0.2.0", map[string][]byte{
		"forge-darwin-arm64.tar.gz": buildTarGz(t, map[string]string{"forge": "new-binary"}),
	})
	defer server.Close()

	updater := New(Config{
		CurrentVersion: "v0.1.0",
		HTTPClient:     server.Client(),
		APIBaseURL:     server.URL,
		ExecutablePath: executablePath,
		GOOS:           "darwin",
		GOARCH:         "arm64",
	})

	result, err := updater.Run(context.Background(), Options{Check: true})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.UpToDate {
		t.Fatal("expected update to be available")
	}
	if result.Updated {
		t.Fatal("expected check-only mode to avoid installation")
	}

	contents, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatalf("read executable: %v", err)
	}
	if string(contents) != originalContents {
		t.Fatalf("expected executable contents to stay %q, got %q", originalContents, string(contents))
	}
}

func TestUpdaterRunInstallsRequestedVersion(t *testing.T) {
	t.Parallel()

	server, executablePath, _ := newTestUpdateServer(t, "v0.2.0", map[string][]byte{
		"forge-darwin-arm64.tar.gz": buildTarGz(t, map[string]string{"forge": "updated-binary"}),
	})
	defer server.Close()

	updater := New(Config{
		CurrentVersion: "v0.1.0",
		HTTPClient:     server.Client(),
		APIBaseURL:     server.URL,
		ExecutablePath: executablePath,
		GOOS:           "darwin",
		GOARCH:         "arm64",
	})

	result, err := updater.Run(context.Background(), Options{Version: "v0.2.0"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Updated {
		t.Fatal("expected install to replace the binary")
	}

	contents, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatalf("read executable: %v", err)
	}
	if string(contents) != "updated-binary" {
		t.Fatalf("expected updated binary contents, got %q", string(contents))
	}
}

func TestUpdaterRunAlreadyCurrent(t *testing.T) {
	t.Parallel()

	server, executablePath, originalContents := newTestUpdateServer(t, "v0.2.0", map[string][]byte{
		"forge-darwin-arm64.tar.gz": buildTarGz(t, map[string]string{"forge": "updated-binary"}),
	})
	defer server.Close()

	updater := New(Config{
		CurrentVersion: "v0.2.0",
		HTTPClient:     server.Client(),
		APIBaseURL:     server.URL,
		ExecutablePath: executablePath,
		GOOS:           "darwin",
		GOARCH:         "arm64",
	})

	result, err := updater.Run(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.UpToDate {
		t.Fatal("expected up-to-date result")
	}
	if result.Updated {
		t.Fatal("did not expect replacement when already current")
	}

	contents, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatalf("read executable: %v", err)
	}
	if string(contents) != originalContents {
		t.Fatalf("expected executable contents to remain %q, got %q", originalContents, string(contents))
	}
}

func TestUpdaterRunPermissionFailure(t *testing.T) {
	t.Parallel()

	server, executablePath, _ := newTestUpdateServer(t, "v0.2.0", map[string][]byte{
		"forge-darwin-arm64.tar.gz": buildTarGz(t, map[string]string{"forge": "updated-binary"}),
	})
	defer server.Close()

	executableDir := filepath.Dir(executablePath)
	if err := os.Chmod(executableDir, 0o555); err != nil {
		t.Fatalf("chmod executable dir: %v", err)
	}
	defer os.Chmod(executableDir, 0o755)

	updater := New(Config{
		CurrentVersion: "v0.1.0",
		HTTPClient:     server.Client(),
		APIBaseURL:     server.URL,
		ExecutablePath: executablePath,
		GOOS:           "darwin",
		GOARCH:         "arm64",
	})

	_, err := updater.Run(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("expected not writable error, got %v", err)
	}
}

func TestUpdaterRunRequestedVersionNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	updater := New(Config{
		CurrentVersion: "v0.1.0",
		HTTPClient:     server.Client(),
		APIBaseURL:     server.URL,
		ExecutablePath: filepath.Join(t.TempDir(), "forge"),
		GOOS:           "darwin",
		GOARCH:         "arm64",
	})

	_, err := updater.Run(context.Background(), Options{Version: "v9.9.9"})
	if err == nil || !strings.Contains(err.Error(), "release v9.9.9 not found") {
		t.Fatalf("expected requested-version-not-found error, got %v", err)
	}
}

func newTestUpdateServer(t *testing.T, tag string, assets map[string][]byte) (*httptest.Server, string, string) {
	t.Helper()

	tempDir := t.TempDir()
	executablePath := filepath.Join(tempDir, "forge")
	originalContents := "current-binary"
	if err := os.WriteFile(executablePath, []byte(originalContents), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	checksumLines := make([]string, 0, len(assets))
	for name, contents := range assets {
		sum := sha256.Sum256(contents)
		checksumLines = append(checksumLines, hex.EncodeToString(sum[:])+" "+name)
	}
	assets[checksumAssetName] = []byte(strings.Join(checksumLines, "\n") + "\n")

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest", "/releases/tags/" + tag:
			releaseAssets := make([]releaseAsset, 0, len(assets))
			for name := range assets {
				releaseAssets = append(releaseAssets, releaseAsset{
					Name:               name,
					BrowserDownloadURL: serverURL + "/downloads/" + name,
				})
			}
			if err := json.NewEncoder(w).Encode(release{
				TagName: tag,
				Assets:  releaseAssets,
			}); err != nil {
				t.Fatalf("encode release: %v", err)
			}
		default:
			name := strings.TrimPrefix(r.URL.Path, "/downloads/")
			contents, ok := assets[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if _, err := w.Write(contents); err != nil {
				t.Fatalf("write asset: %v", err)
			}
		}
	}))
	serverURL = server.URL

	return server, executablePath, originalContents
}

func buildTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for name, contents := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(contents)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write([]byte(contents)); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buffer.Bytes()
}
