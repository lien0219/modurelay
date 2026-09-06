package service

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrNoUpdateAvailable         = infraerrors.Conflict("ALREADY_UP_TO_DATE", "no update available; current version is latest")
	ErrUpdateCheckUnavailable    = infraerrors.ServiceUnavailable("UPDATE_CHECK_UNAVAILABLE", "version source could not be verified; update was not started")
	ErrRollbackVersionNotAllowed = infraerrors.BadRequest("ROLLBACK_VERSION_NOT_ALLOWED", "version is not in the allowed rollback list")
	ErrHostDeploymentRequired    = infraerrors.Conflict("HOST_DEPLOYMENT_REQUIRED", "Docker deployments must be changed on the host with deploy-main.sh")
	ErrInPlaceUpdateUnavailable  = infraerrors.Conflict("IN_PLACE_UPDATE_UNAVAILABLE", "in-place update and rollback are available only for standalone release binaries")
)

const (
	updateCacheKey = "update_check_cache"
	updateCacheTTL = 1200 // 20 minutes
	githubRepo     = "lien0219/modurelay"
	ghcrRepository = "lien0219/modurelay"
	ghcrImage      = "ghcr.io/" + ghcrRepository
	ghcrPackageURL = "https://github.com/lien0219/modurelay/pkgs/container/modurelay"

	// Security: allowed download domains for updates
	allowedDownloadHost = "github.com"
	allowedAssetHost    = "objects.githubusercontent.com"

	// Security: max download size (500MB)
	maxDownloadSize = 500 * 1024 * 1024

	// Rollback: expose at most the 3 most recent versions older than current
	maxRollbackVersions = 3
	// Fetch a few extra releases so filtering (current/newer/prerelease) still leaves enough candidates
	rollbackFetchPageSize = 15

	DeploymentModeSource = "source"
	DeploymentModeBinary = "binary"
	DeploymentModeDocker = "docker"

	RollbackMethodBinary      = "binary"
	RollbackMethodHostCommand = "host_command"
)

var (
	semanticVersionPattern = regexp.MustCompile(`(?i)v?([0-9]+\.[0-9]+\.[0-9]+)`)
	releaseVersionPattern  = regexp.MustCompile(`^v?((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*))$`)
	ghcrVersionTagPattern  = regexp.MustCompile(`^main-v((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*))$`)
)

// UpdateCache defines cache operations for update service
type UpdateCache interface {
	GetUpdateInfo(ctx context.Context) (string, error)
	SetUpdateInfo(ctx context.Context, data string, ttl time.Duration) error
}

// GitHubReleaseClient 获取 GitHub release 信息的接口
type GitHubReleaseClient interface {
	FetchLatestRelease(ctx context.Context, repo string) (*GitHubRelease, error)
	FetchRecentReleases(ctx context.Context, repo string, perPage int) ([]*GitHubRelease, error)
	FetchContainerTags(ctx context.Context, repository string) ([]string, error)
	DownloadFile(ctx context.Context, url, dest string, maxSize int64) error
	FetchChecksumFile(ctx context.Context, url string) ([]byte, error)
}

// UpdateService handles software updates
type UpdateService struct {
	cache          UpdateCache
	githubClient   GitHubReleaseClient
	currentVersion string
	buildType      string // "source" for manual builds, "release" for CI builds
	deploymentMode string // "source", "binary", or "docker"
}

// NewUpdateService creates a new UpdateService
func NewUpdateService(cache UpdateCache, githubClient GitHubReleaseClient, version, buildType, deploymentMode string) *UpdateService {
	return &UpdateService{
		cache:          cache,
		githubClient:   githubClient,
		currentVersion: version,
		buildType:      buildType,
		deploymentMode: normalizeDeploymentMode(deploymentMode, buildType),
	}
}

// UpdateInfo contains update information
type UpdateInfo struct {
	CurrentVersion string       `json:"current_version"`
	LatestVersion  string       `json:"latest_version"`
	HasUpdate      bool         `json:"has_update"`
	ReleaseInfo    *ReleaseInfo `json:"release_info,omitempty"`
	Cached         bool         `json:"cached"`
	Warning        string       `json:"warning,omitempty"`
	BuildType      string       `json:"build_type"` // "source" or "release"
	DeploymentMode string       `json:"deployment_mode"`
	TargetImage    string       `json:"target_image,omitempty"`
	DeployCommand  string       `json:"deploy_command,omitempty"`
}

// ReleaseInfo contains GitHub release details
type ReleaseInfo struct {
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	PublishedAt string  `json:"published_at"`
	HTMLURL     string  `json:"html_url"`
	Assets      []Asset `json:"assets,omitempty"`
}

// Asset represents a release asset
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
}

// GitHubRelease represents GitHub API response
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	Assets      []GitHubAsset `json:"assets"`
}

// RollbackVersion describes a release version the system can roll back to
type RollbackVersion struct {
	Version       string `json:"version"` // without "v" prefix, e.g. "0.1.146"
	PublishedAt   string `json:"published_at,omitempty"`
	HTMLURL       string `json:"html_url,omitempty"`
	Image         string `json:"image,omitempty"`
	DeployCommand string `json:"deploy_command,omitempty"`
	Method        string `json:"method"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	APIURL             string `json:"url"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckUpdate checks for available updates
func (s *UpdateService) CheckUpdate(ctx context.Context, force bool) (*UpdateInfo, error) {
	// Try cache first
	if !force {
		if cached, err := s.getFromCache(ctx); err == nil && cached != nil {
			return cached, nil
		}
	}

	var info *UpdateInfo
	var err error
	if s.deploymentMode == DeploymentModeDocker {
		info, err = s.fetchLatestContainerImage(ctx)
	} else {
		info, err = s.fetchLatestRelease(ctx)
	}
	if err != nil {
		// Return cached on error
		if cached, cacheErr := s.getFromCache(ctx); cacheErr == nil && cached != nil {
			cached.Warning = "Using cached data: " + err.Error()
			return cached, nil
		}
		return &UpdateInfo{
			CurrentVersion: s.currentVersion,
			LatestVersion:  s.currentVersion,
			HasUpdate:      false,
			Warning:        err.Error(),
			BuildType:      s.buildType,
			DeploymentMode: s.deploymentMode,
		}, nil
	}

	// Cache result
	s.saveToCache(ctx, info)
	return info, nil
}

// PerformUpdate downloads and applies the update
// Uses atomic file replacement pattern for safe in-place updates
func (s *UpdateService) PerformUpdate(ctx context.Context) error {
	if s.deploymentMode == DeploymentModeDocker {
		return ErrHostDeploymentRequired
	}
	if s.deploymentMode != DeploymentModeBinary {
		return ErrInPlaceUpdateUnavailable
	}

	info, err := s.CheckUpdate(ctx, true)
	if err != nil {
		return err
	}
	if info.Warning != "" {
		return ErrUpdateCheckUnavailable
	}

	if !info.HasUpdate {
		return ErrNoUpdateAvailable
	}

	return s.applyReleaseAssets(ctx, info.ReleaseInfo.Assets)
}

// applyReleaseAssets downloads the platform archive from the given release assets,
// verifies its checksum, and atomically swaps the running binary.
// Shared by PerformUpdate (latest) and RollbackToVersion (specific older version).
func (s *UpdateService) applyReleaseAssets(ctx context.Context, releaseAssets []Asset) error {
	archiveAssetName, downloadURL, checksumURL, err := s.selectReleaseAssets(releaseAssets)
	if err != nil {
		return err
	}

	// SECURITY: Validate download URL is from trusted domain
	if err := validateDownloadURL(downloadURL); err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	if checksumURL != "" {
		if err := validateDownloadURL(checksumURL); err != nil {
			return fmt.Errorf("invalid checksum URL: %w", err)
		}
	}

	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	exeDir := filepath.Dir(exePath)

	// Create temp directory in the SAME directory as executable
	// This ensures os.Rename is atomic (same filesystem)
	tempDir, err := os.MkdirTemp(exeDir, ".sub2api-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Download archive
	archivePath := filepath.Join(tempDir, filepath.Base(archiveAssetName))
	if err := s.downloadFile(ctx, downloadURL, archivePath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify checksum if available
	if checksumURL != "" {
		if err := s.verifyChecksum(ctx, archivePath, checksumURL); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Extract binary from archive
	newBinaryPath := filepath.Join(tempDir, "sub2api")
	if err := s.extractBinary(archivePath, newBinaryPath); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Set executable permission before replacement
	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	// Atomic replacement using rename pattern:
	// 1. Rename current -> backup (atomic on Unix)
	// 2. Rename new -> current (atomic on Unix, same filesystem)
	// If step 2 fails, restore backup
	backupPath := exePath + ".backup"

	// Remove old backup if exists
	_ = os.Remove(backupPath)

	// Step 1: Move current binary to backup
	if err := os.Rename(exePath, backupPath); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Step 2: Move new binary to target location (atomic, same filesystem)
	if err := os.Rename(newBinaryPath, exePath); err != nil {
		// Restore backup on failure
		if restoreErr := os.Rename(backupPath, exePath); restoreErr != nil {
			return fmt.Errorf("replace failed and restore failed: %w (restore error: %v)", err, restoreErr)
		}
		return fmt.Errorf("replace failed (restored backup): %w", err)
	}

	// Success - backup file is kept for rollback capability
	// It will be cleaned up on next successful update
	return nil
}

func (s *UpdateService) selectReleaseAssets(releaseAssets []Asset) (archiveName, downloadURL, checksumURL string, err error) {
	platformName := s.getArchiveName()
	for _, asset := range releaseAssets {
		if strings.Contains(asset.Name, platformName) && !strings.HasSuffix(asset.Name, ".txt") {
			archiveName = asset.Name
			downloadURL = asset.DownloadURL
		}
		if asset.Name == "checksums.txt" {
			checksumURL = asset.DownloadURL
		}
	}
	if downloadURL == "" {
		return "", "", "", fmt.Errorf("no compatible release found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if checksumURL == "" {
		return "", "", "", fmt.Errorf("checksums.txt is required for in-place update and rollback")
	}
	return archiveName, downloadURL, checksumURL, nil
}

// Rollback restores the previous version
func (s *UpdateService) Rollback() error {
	if s.deploymentMode == DeploymentModeDocker {
		return ErrHostDeploymentRequired
	}
	if s.deploymentMode != DeploymentModeBinary {
		return ErrInPlaceUpdateUnavailable
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	backupFile := exePath + ".backup"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("no backup found")
	}

	// Replace current with backup
	if err := os.Rename(backupFile, exePath); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	return nil
}

// ListRollbackVersions returns up to maxRollbackVersions release versions that are
// strictly older than the current version (the current version itself is excluded),
// newest first. Draft and prerelease entries are skipped.
func (s *UpdateService) ListRollbackVersions(ctx context.Context) ([]RollbackVersion, error) {
	if s.deploymentMode == DeploymentModeDocker {
		return s.listDockerRollbackVersions(ctx)
	}
	if s.deploymentMode != DeploymentModeBinary {
		return []RollbackVersion{}, nil
	}

	releases, err := s.fetchRollbackCandidates(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]RollbackVersion, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, RollbackVersion{
			Version:     strings.TrimPrefix(r.TagName, "v"),
			PublishedAt: r.PublishedAt,
			HTMLURL:     r.HTMLURL,
			Method:      RollbackMethodBinary,
		})
	}
	return versions, nil
}

// RollbackToVersion downloads and installs a specific older version.
// The target must be one of the versions returned by ListRollbackVersions;
// anything else (including the current version) is rejected.
func (s *UpdateService) RollbackToVersion(ctx context.Context, version string) error {
	target := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if target == "" {
		return ErrRollbackVersionNotAllowed
	}
	if s.deploymentMode == DeploymentModeDocker {
		return ErrHostDeploymentRequired
	}
	if s.deploymentMode != DeploymentModeBinary {
		return ErrInPlaceUpdateUnavailable
	}

	releases, err := s.fetchRollbackCandidates(ctx)
	if err != nil {
		return err
	}

	var match *GitHubRelease
	for _, r := range releases {
		if strings.TrimPrefix(r.TagName, "v") == target {
			match = r
			break
		}
	}
	if match == nil {
		return ErrRollbackVersionNotAllowed
	}

	assets := make([]Asset, len(match.Assets))
	for i, a := range match.Assets {
		downloadURL := a.APIURL
		if downloadURL == "" {
			downloadURL = a.BrowserDownloadURL
		}
		assets[i] = Asset{
			Name:        a.Name,
			DownloadURL: downloadURL,
			Size:        a.Size,
		}
	}

	return s.applyReleaseAssets(ctx, assets)
}

// fetchRollbackCandidates fetches recent releases and keeps the newest
// maxRollbackVersions entries strictly older than the current version.
func (s *UpdateService) fetchRollbackCandidates(ctx context.Context) ([]*GitHubRelease, error) {
	releases, err := s.githubClient.FetchRecentReleases(ctx, githubRepo, rollbackFetchPageSize)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(releases))
	candidates := make([]*GitHubRelease, 0, maxRollbackVersions)
	for _, r := range releases {
		if r == nil || r.Draft || r.Prerelease {
			continue
		}
		match := releaseVersionPattern.FindStringSubmatch(strings.TrimSpace(r.TagName))
		if len(match) < 2 {
			continue
		}
		v := match[1]
		if seen[v] {
			continue
		}
		// Only versions strictly older than current (also excludes current itself)
		if compareVersions(v, s.currentVersion) >= 0 {
			continue
		}
		seen[v] = true
		candidates = append(candidates, r)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareVersions(
			strings.TrimPrefix(candidates[i].TagName, "v"),
			strings.TrimPrefix(candidates[j].TagName, "v"),
		) > 0
	})

	if len(candidates) > maxRollbackVersions {
		candidates = candidates[:maxRollbackVersions]
	}
	return candidates, nil
}

func (s *UpdateService) listDockerRollbackVersions(ctx context.Context) ([]RollbackVersion, error) {
	versions, err := s.fetchContainerVersions(ctx)
	if err != nil {
		return nil, err
	}

	current := normalizeSemanticVersion(s.currentVersion)
	if current == "" {
		return nil, fmt.Errorf("current Docker image version %q is not semantic", s.currentVersion)
	}

	result := make([]RollbackVersion, 0, maxRollbackVersions)
	for _, version := range versions {
		if compareVersions(version, current) >= 0 {
			continue
		}
		image := ghcrImage + ":main-v" + version
		result = append(result, RollbackVersion{
			Version:       version,
			HTMLURL:       ghcrPackageURL,
			Image:         image,
			DeployCommand: "./deploy-main.sh " + image,
			Method:        RollbackMethodHostCommand,
		})
		if len(result) == maxRollbackVersions {
			break
		}
	}
	return result, nil
}

func (s *UpdateService) fetchContainerVersions(ctx context.Context) ([]string, error) {
	tags, err := s.githubClient.FetchContainerTags(ctx, ghcrRepository)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(tags))
	versions := make([]string, 0, len(tags))
	for _, tag := range tags {
		match := ghcrVersionTagPattern.FindStringSubmatch(strings.TrimSpace(tag))
		if len(match) < 2 {
			continue
		}
		version := match[1]
		if _, exists := seen[version]; exists {
			continue
		}
		seen[version] = struct{}{}
		versions = append(versions, version)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no production image tags matching main-vX.Y.Z were found in %s", ghcrImage)
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})
	return versions, nil
}

func (s *UpdateService) fetchLatestContainerImage(ctx context.Context) (*UpdateInfo, error) {
	versions, err := s.fetchContainerVersions(ctx)
	if err != nil {
		return nil, err
	}

	latestVersion := versions[0]
	targetImage := ghcrImage + ":main-v" + latestVersion
	return &UpdateInfo{
		CurrentVersion: s.currentVersion,
		LatestVersion:  latestVersion,
		HasUpdate:      compareVersions(s.currentVersion, latestVersion) < 0,
		ReleaseInfo: &ReleaseInfo{
			Name:    "ModuRelay main-v" + latestVersion,
			HTMLURL: ghcrPackageURL,
		},
		Cached:         false,
		BuildType:      s.buildType,
		DeploymentMode: s.deploymentMode,
		TargetImage:    targetImage,
		DeployCommand:  "./deploy-main.sh " + targetImage,
	}, nil
}

func (s *UpdateService) fetchLatestRelease(ctx context.Context) (*UpdateInfo, error) {
	release, err := s.githubClient.FetchLatestRelease(ctx, githubRepo)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, fmt.Errorf("GitHub release response was empty")
	}

	match := releaseVersionPattern.FindStringSubmatch(strings.TrimSpace(release.TagName))
	if len(match) < 2 {
		return nil, fmt.Errorf("latest GitHub release tag %q is not a stable semantic version", release.TagName)
	}
	latestVersion := match[1]

	assets := make([]Asset, len(release.Assets))
	for i, a := range release.Assets {
		downloadURL := a.APIURL
		if downloadURL == "" {
			downloadURL = a.BrowserDownloadURL
		}
		assets[i] = Asset{
			Name:        a.Name,
			DownloadURL: downloadURL,
			Size:        a.Size,
		}
	}

	return &UpdateInfo{
		CurrentVersion: s.currentVersion,
		LatestVersion:  latestVersion,
		HasUpdate:      compareVersions(s.currentVersion, latestVersion) < 0,
		ReleaseInfo: &ReleaseInfo{
			Name:        release.Name,
			Body:        release.Body,
			PublishedAt: release.PublishedAt,
			HTMLURL:     release.HTMLURL,
			Assets:      assets,
		},
		Cached:         false,
		BuildType:      s.buildType,
		DeploymentMode: s.deploymentMode,
	}, nil
}

func (s *UpdateService) downloadFile(ctx context.Context, downloadURL, dest string) error {
	return s.githubClient.DownloadFile(ctx, downloadURL, dest, maxDownloadSize)
}

func (s *UpdateService) getArchiveName() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	return fmt.Sprintf("%s_%s", osName, arch)
}

// validateDownloadURL checks if the URL is from an allowed domain
// SECURITY: This prevents SSRF and ensures downloads only come from trusted GitHub domains
func validateDownloadURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Must be HTTPS
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTPS URLs are allowed")
	}

	// Check against allowed hosts
	host := parsedURL.Host
	// GitHub release URLs can be from github.com or objects.githubusercontent.com
	if host != allowedDownloadHost &&
		!strings.HasSuffix(host, "."+allowedDownloadHost) &&
		host != allowedAssetHost &&
		!strings.HasSuffix(host, "."+allowedAssetHost) {
		return fmt.Errorf("download from untrusted host: %s", host)
	}

	return nil
}

func (s *UpdateService) verifyChecksum(ctx context.Context, filePath, checksumURL string) error {
	// Download checksums file
	checksumData, err := s.githubClient.FetchChecksumFile(ctx, checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}

	// Calculate file hash
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	// Find expected hash in checksums file
	fileName := filepath.Base(filePath)
	scanner := bufio.NewScanner(strings.NewReader(string(checksumData)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == fileName {
			if parts[0] == actualHash {
				return nil
			}
			return fmt.Errorf("checksum mismatch: expected %s, got %s", parts[0], actualHash)
		}
	}

	return fmt.Errorf("checksum not found for %s", fileName)
}

func (s *UpdateService) extractBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var reader io.Reader = f

	// Handle gzip compression
	if strings.HasSuffix(archivePath, ".gz") || strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer func() { _ = gzr.Close() }()
		reader = gzr
	}

	// Handle tar archive
	if strings.Contains(archivePath, ".tar") {
		tr := tar.NewReader(reader)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			// SECURITY: Prevent Zip Slip / Path Traversal attack
			// Only allow files with safe base names, no directory traversal
			baseName := filepath.Base(hdr.Name)

			// Check for path traversal attempts
			if strings.Contains(hdr.Name, "..") {
				return fmt.Errorf("path traversal attempt detected: %s", hdr.Name)
			}

			// Validate the entry is a regular file
			if hdr.Typeflag != tar.TypeReg {
				continue // Skip directories and special files
			}

			// Only extract the specific binary we need
			if baseName == "sub2api" || baseName == "sub2api.exe" {
				// Additional security: limit file size (max 500MB)
				const maxBinarySize = 500 * 1024 * 1024
				if hdr.Size > maxBinarySize {
					return fmt.Errorf("binary too large: %d bytes (max %d)", hdr.Size, maxBinarySize)
				}

				out, err := os.Create(destPath)
				if err != nil {
					return err
				}

				// Use LimitReader to prevent decompression bombs
				limited := io.LimitReader(tr, maxBinarySize)
				if _, err := io.Copy(out, limited); err != nil {
					_ = out.Close()
					return err
				}
				if err := out.Close(); err != nil {
					return err
				}
				return nil
			}
		}
		return fmt.Errorf("binary not found in archive")
	}

	// Direct copy for non-tar files (with size limit)
	const maxBinarySize = 500 * 1024 * 1024
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}

	limited := io.LimitReader(reader, maxBinarySize)
	if _, err := io.Copy(out, limited); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func (s *UpdateService) getFromCache(ctx context.Context) (*UpdateInfo, error) {
	data, err := s.cache.GetUpdateInfo(ctx)
	if err != nil {
		return nil, err
	}

	var cached struct {
		Latest         string       `json:"latest"`
		ReleaseInfo    *ReleaseInfo `json:"release_info"`
		DeploymentMode string       `json:"deployment_mode"`
		TargetImage    string       `json:"target_image"`
		DeployCommand  string       `json:"deploy_command"`
		Timestamp      int64        `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		return nil, err
	}

	if time.Now().Unix()-cached.Timestamp > updateCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}
	if cached.DeploymentMode != s.deploymentMode {
		return nil, fmt.Errorf("cache deployment mode mismatch")
	}

	return &UpdateInfo{
		CurrentVersion: s.currentVersion,
		LatestVersion:  cached.Latest,
		HasUpdate:      compareVersions(s.currentVersion, cached.Latest) < 0,
		ReleaseInfo:    cached.ReleaseInfo,
		Cached:         true,
		BuildType:      s.buildType,
		DeploymentMode: s.deploymentMode,
		TargetImage:    cached.TargetImage,
		DeployCommand:  cached.DeployCommand,
	}, nil
}

func (s *UpdateService) saveToCache(ctx context.Context, info *UpdateInfo) {
	cacheData := struct {
		Latest         string       `json:"latest"`
		ReleaseInfo    *ReleaseInfo `json:"release_info"`
		DeploymentMode string       `json:"deployment_mode"`
		TargetImage    string       `json:"target_image"`
		DeployCommand  string       `json:"deploy_command"`
		Timestamp      int64        `json:"timestamp"`
	}{
		Latest:         info.LatestVersion,
		ReleaseInfo:    info.ReleaseInfo,
		DeploymentMode: info.DeploymentMode,
		TargetImage:    info.TargetImage,
		DeployCommand:  info.DeployCommand,
		Timestamp:      time.Now().Unix(),
	}

	data, _ := json.Marshal(cacheData)
	_ = s.cache.SetUpdateInfo(ctx, string(data), time.Duration(updateCacheTTL)*time.Second)
}

// compareVersions compares two semantic versions
func compareVersions(current, latest string) int {
	currentParts := parseVersion(current)
	latestParts := parseVersion(latest)

	for i := 0; i < 3; i++ {
		if currentParts[i] < latestParts[i] {
			return -1
		}
		if currentParts[i] > latestParts[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = normalizeSemanticVersion(v)
	parts := strings.Split(v, ".")
	result := [3]int{0, 0, 0}
	for i := 0; i < len(parts) && i < 3; i++ {
		if parsed, err := strconv.Atoi(parts[i]); err == nil {
			result[i] = parsed
		}
	}
	return result
}

func normalizeSemanticVersion(v string) string {
	match := semanticVersionPattern.FindStringSubmatch(strings.TrimSpace(v))
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func normalizeDeploymentMode(mode, buildType string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case DeploymentModeSource, DeploymentModeBinary, DeploymentModeDocker:
		return strings.ToLower(strings.TrimSpace(mode))
	}
	if strings.EqualFold(strings.TrimSpace(buildType), "release") {
		return DeploymentModeBinary
	}
	return DeploymentModeSource
}
