package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultAPIBaseURL = "https://api.github.com/repos/emkaytec/forge"
	binaryName        = "forge"
	checksumAssetName = "SHA256SUMS.txt"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Options struct {
	Check   bool
	Version string
}

type Result struct {
	CurrentVersion string
	TargetVersion  string
	AssetName      string
	Updated        bool
	UpToDate       bool
}

type Config struct {
	CurrentVersion string
	HTTPClient     HTTPClient
	APIBaseURL     string
	ExecutablePath string
	GOOS           string
	GOARCH         string
}

type Updater struct {
	currentVersion string
	httpClient     HTTPClient
	apiBaseURL     string
	executablePath string
	goos           string
	goarch         string
}

type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type platform struct {
	GOOS   string
	GOARCH string
}

func New(config Config) *Updater {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	apiBaseURL := strings.TrimRight(config.APIBaseURL, "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}

	return &Updater{
		currentVersion: config.CurrentVersion,
		httpClient:     httpClient,
		apiBaseURL:     apiBaseURL,
		executablePath: config.ExecutablePath,
		goos:           valueOrDefault(config.GOOS, runtime.GOOS),
		goarch:         valueOrDefault(config.GOARCH, runtime.GOARCH),
	}
}

func (u *Updater) Run(ctx context.Context, opts Options) (Result, error) {
	resolvedPlatform, err := resolvePlatform(u.goos, u.goarch)
	if err != nil {
		return Result{}, err
	}

	targetRelease, err := u.lookupRelease(ctx, strings.TrimSpace(opts.Version))
	if err != nil {
		return Result{}, err
	}

	targetAssetName := archiveNameForPlatform(resolvedPlatform)
	targetAsset, err := findAsset(targetRelease.Assets, targetAssetName)
	if err != nil {
		return Result{}, err
	}

	checksumAsset, err := findAsset(targetRelease.Assets, checksumAssetName)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		CurrentVersion: u.currentVersion,
		TargetVersion:  targetRelease.TagName,
		AssetName:      targetAsset.Name,
	}

	if shouldSkipInstall(u.currentVersion, targetRelease.TagName, strings.TrimSpace(opts.Version) != "") {
		result.UpToDate = true
		return result, nil
	}

	if opts.Check {
		return result, nil
	}

	executablePath, err := u.resolveExecutablePath()
	if err != nil {
		return Result{}, err
	}

	if err := ensureWritableDestination(executablePath); err != nil {
		return Result{}, err
	}

	tempDir, err := os.MkdirTemp("", "forge-update-*")
	if err != nil {
		return Result{}, fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, targetAsset.Name)
	if err := u.downloadToFile(ctx, targetAsset.BrowserDownloadURL, archivePath); err != nil {
		return Result{}, err
	}

	checksumPath := filepath.Join(tempDir, checksumAsset.Name)
	if err := u.downloadToFile(ctx, checksumAsset.BrowserDownloadURL, checksumPath); err != nil {
		return Result{}, err
	}

	if err := verifyChecksum(archivePath, checksumPath, targetAsset.Name); err != nil {
		return Result{}, err
	}

	extractedBinaryPath := filepath.Join(tempDir, binaryName)
	if err := extractBinary(archivePath, extractedBinaryPath); err != nil {
		return Result{}, err
	}

	if err := replaceExecutable(executablePath, extractedBinaryPath); err != nil {
		return Result{}, err
	}

	result.Updated = true
	return result, nil
}

func resolvePlatform(goos, goarch string) (platform, error) {
	switch {
	case goos == "darwin" && (goarch == "amd64" || goarch == "arm64"):
		return platform{GOOS: goos, GOARCH: goarch}, nil
	case goos == "linux" && (goarch == "amd64" || goarch == "arm64"):
		return platform{GOOS: goos, GOARCH: goarch}, nil
	default:
		return platform{}, fmt.Errorf("unsupported platform %s/%s", goos, goarch)
	}
}

func archiveNameForPlatform(p platform) string {
	return fmt.Sprintf("%s-%s-%s.tar.gz", binaryName, p.GOOS, p.GOARCH)
}

func (u *Updater) lookupRelease(ctx context.Context, version string) (release, error) {
	endpoint := u.apiBaseURL + "/releases/latest"
	notFoundMessage := "no releases found"
	if version != "" {
		endpoint = u.apiBaseURL + "/releases/tags/" + version
		notFoundMessage = fmt.Sprintf("release %s not found", version)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return release{}, fmt.Errorf("build release lookup request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("lookup release metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return release{}, errors.New(notFoundMessage)
	}
	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("lookup release metadata: unexpected status %s", resp.Status)
	}

	var target release
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		return release{}, fmt.Errorf("decode release metadata: %w", err)
	}
	if strings.TrimSpace(target.TagName) == "" {
		return release{}, errors.New("release metadata did not include a tag")
	}

	return target, nil
}

func findAsset(assets []releaseAsset, name string) (releaseAsset, error) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, nil
		}
	}

	return releaseAsset{}, fmt.Errorf("release asset %s not found", name)
}

func (u *Updater) resolveExecutablePath() (string, error) {
	if strings.TrimSpace(u.executablePath) != "" {
		return u.executablePath, nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable path: %w", err)
	}
	if strings.TrimSpace(executablePath) == "" {
		return "", errors.New("resolve current executable path: empty path")
	}

	return executablePath, nil
}

func ensureWritableDestination(executablePath string) error {
	dir := filepath.Dir(executablePath)
	testFile, err := os.CreateTemp(dir, ".forge-update-*")
	if err != nil {
		return fmt.Errorf("executable path %s is not writable: %w", executablePath, err)
	}
	testFile.Close()
	os.Remove(testFile.Name())
	return nil
}

func (u *Updater) downloadToFile(ctx context.Context, url, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request for %s: %w", path.Base(destination), err)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", path.Base(destination), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", path.Base(destination), resp.Status)
	}

	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}

	return nil
}

func verifyChecksum(archivePath, checksumPath, assetName string) error {
	expectedChecksum, err := expectedChecksumForAsset(checksumPath, assetName)
	if err != nil {
		return err
	}

	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive for checksum verification: %w", err)
	}
	defer archive.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actualChecksum, expectedChecksum) {
		return fmt.Errorf("checksum verification failed for %s", assetName)
	}

	return nil
}

func expectedChecksumForAsset(checksumPath, assetName string) (string, error) {
	content, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", fmt.Errorf("read checksum file: %w", err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == assetName {
			return fields[0], nil
		}
	}

	return "", fmt.Errorf("checksum entry for %s not found", assetName)
}

func extractBinary(archivePath, destination string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer archive.Close()

	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("open gzip archive: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive entry: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if path.Base(header.Name) != binaryName {
			continue
		}

		file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return fmt.Errorf("create extracted binary: %w", err)
		}

		if _, err := io.Copy(file, tarReader); err != nil {
			file.Close()
			return fmt.Errorf("extract binary from archive: %w", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close extracted binary: %w", err)
		}

		return nil
	}

	return fmt.Errorf("archive did not contain %s binary", binaryName)
}

func replaceExecutable(destinationPath, extractedBinaryPath string) error {
	destinationDir := filepath.Dir(destinationPath)
	tempDestination, err := os.CreateTemp(destinationDir, ".forge-update-*")
	if err != nil {
		return fmt.Errorf("create replacement binary: %w", err)
	}
	tempDestinationPath := tempDestination.Name()

	source, err := os.Open(extractedBinaryPath)
	if err != nil {
		tempDestination.Close()
		os.Remove(tempDestinationPath)
		return fmt.Errorf("open extracted binary: %w", err)
	}
	defer source.Close()

	if _, err := io.Copy(tempDestination, source); err != nil {
		tempDestination.Close()
		os.Remove(tempDestinationPath)
		return fmt.Errorf("write replacement binary: %w", err)
	}
	if err := tempDestination.Chmod(0o755); err != nil {
		tempDestination.Close()
		os.Remove(tempDestinationPath)
		return fmt.Errorf("chmod replacement binary: %w", err)
	}
	if err := tempDestination.Close(); err != nil {
		os.Remove(tempDestinationPath)
		return fmt.Errorf("close replacement binary: %w", err)
	}

	if err := os.Rename(tempDestinationPath, destinationPath); err != nil {
		os.Remove(tempDestinationPath)
		return fmt.Errorf("replace executable: %w", err)
	}

	return nil
}

func shouldSkipInstall(currentVersion, targetVersion string, pinnedVersion bool) bool {
	currentVersion = strings.TrimSpace(currentVersion)
	targetVersion = strings.TrimSpace(targetVersion)
	if currentVersion == "" || targetVersion == "" {
		return false
	}
	if currentVersion == targetVersion {
		return true
	}
	if pinnedVersion {
		return false
	}

	currentSemver, currentOK := parseSemver(currentVersion)
	targetSemver, targetOK := parseSemver(targetVersion)
	if !currentOK || !targetOK {
		return false
	}

	return compareSemver(currentSemver, targetSemver) >= 0
}

type semver struct {
	Major int
	Minor int
	Patch int
}

func parseSemver(value string) (semver, bool) {
	if !strings.HasPrefix(value, "v") {
		return semver{}, false
	}

	parts := strings.Split(strings.TrimPrefix(value, "v"), ".")
	if len(parts) != 3 {
		return semver{}, false
	}

	var parsed semver
	if _, err := fmt.Sscanf(value, "v%d.%d.%d", &parsed.Major, &parsed.Minor, &parsed.Patch); err != nil {
		return semver{}, false
	}

	return parsed, true
}

func compareSemver(left, right semver) int {
	switch {
	case left.Major != right.Major:
		return compareInt(left.Major, right.Major)
	case left.Minor != right.Minor:
		return compareInt(left.Minor, right.Minor)
	default:
		return compareInt(left.Patch, right.Patch)
	}
}

func compareInt(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}

	return fallback
}
