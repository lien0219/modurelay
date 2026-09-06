//go:build unit

package service

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release        *GitHubRelease
	recentReleases []*GitHubRelease
	recentErr      error
	containerTags  []string
	containerErr   error
	downloadErr    error
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) FetchRecentReleases(context.Context, string, int) ([]*GitHubRelease, error) {
	return s.recentReleases, s.recentErr
}

func (s *updateServiceGitHubClientStub) FetchContainerTags(context.Context, string) ([]string, error) {
	return s.containerTags, s.containerErr
}

func (s *updateServiceGitHubClientStub) DownloadFile(_ context.Context, _ string, dest string, _ int64) error {
	return s.downloadErr
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"release",
		"binary",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

func TestUpdateServicePerformUpdateDoesNotReportUpToDateWhenCheckFailed(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{},
		"0.3.0",
		"release",
		"binary",
	)

	err := svc.PerformUpdate(context.Background())

	require.ErrorIs(t, err, ErrUpdateCheckUnavailable)
}

func newRollbackTestService(current string, releases []*GitHubRelease) *UpdateService {
	return NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{recentReleases: releases},
		current,
		"release",
		"binary",
	)
}

func TestUpdateServiceListRollbackVersionsFiltersAndCaps(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.148", PublishedAt: "2026-07-09T00:00:00Z"},                       // newer than current: excluded
		{TagName: "v0.1.147", PublishedAt: "2026-07-08T00:00:00Z"},                       // current: excluded
		{TagName: "v0.1.146-rc1", PublishedAt: "2026-07-07T12:00:00Z", Prerelease: true}, // prerelease: excluded
		{TagName: "v0.1.146", PublishedAt: "2026-07-07T00:00:00Z"},
		{TagName: "v0.1.145", PublishedAt: "2026-07-06T00:00:00Z", Draft: true}, // draft: excluded
		{TagName: "v0.1.144", PublishedAt: "2026-07-05T00:00:00Z"},
		{TagName: "v0.1.144", PublishedAt: "2026-07-05T00:00:00Z"}, // duplicate: excluded
		{TagName: "v0.1.143", PublishedAt: "2026-07-04T00:00:00Z"},
		{TagName: "v0.1.142", PublishedAt: "2026-07-03T00:00:00Z"}, // beyond cap of 3: excluded
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, "0.1.146", versions[0].Version)
	require.Equal(t, "0.1.144", versions[1].Version)
	require.Equal(t, "0.1.143", versions[2].Version)
}

func TestUpdateServiceListRollbackVersionsSortsUnorderedInput(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.144"},
		{TagName: "v0.1.146"},
		{TagName: "v0.1.145"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, "0.1.146", versions[0].Version)
	require.Equal(t, "0.1.145", versions[1].Version)
	require.Equal(t, "0.1.144", versions[2].Version)
}

func TestUpdateServiceListRollbackVersionsEmptyWhenNoneOlder(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.147"},
		{TagName: "v0.1.148"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Empty(t, versions)
}

func TestUpdateServiceListRollbackVersionsPropagatesFetchError(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{recentErr: errors.New("github unavailable")},
		"0.1.147",
		"release",
		"binary",
	)

	_, err := svc.ListRollbackVersions(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "github unavailable")
}

func TestUpdateServiceRollbackToVersionRejectsDisallowedTargets(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.148"},
		{TagName: "v0.1.147"},
		{TagName: "v0.1.146"},
		{TagName: "v0.1.145"},
		{TagName: "v0.1.144"},
		{TagName: "v0.1.143"},
		{TagName: "v0.1.142"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	for _, target := range []string{
		"",         // empty
		"0.1.147",  // current version
		"v0.1.147", // current version with prefix
		"0.1.148",  // newer than current
		"0.1.142",  // older than the 3 most recent
		"9.9.9",    // nonexistent
	} {
		err := svc.RollbackToVersion(context.Background(), target)
		require.ErrorIs(t, err, ErrRollbackVersionNotAllowed, "target %q should be rejected", target)
	}
}

func TestUpdateServiceRollbackToVersionAcceptsVPrefix(t *testing.T) {
	// No platform asset in the release: the target passes the allowlist check
	// and fails later at asset lookup, proving the version itself was accepted.
	releases := []*GitHubRelease{
		{TagName: "v0.1.147"},
		{TagName: "v0.1.146"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	err := svc.RollbackToVersion(context.Background(), "v0.1.146")

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrRollbackVersionNotAllowed)
	require.Contains(t, err.Error(), "no compatible release found")
}

func TestUpdateServicePrivateAssetAPIPreservesArchiveName(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{},
		"0.3.1",
		"release",
		"binary",
	)
	assetName := "modurelay_0.3.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"

	archiveName, downloadURL, checksumURL, err := svc.selectReleaseAssets([]Asset{
		{
			Name:        assetName,
			DownloadURL: "https://api.github.com/repos/lien0219/modurelay/releases/assets/123",
		},
		{
			Name:        "checksums.txt",
			DownloadURL: "https://api.github.com/repos/lien0219/modurelay/releases/assets/124",
		},
	})

	require.NoError(t, err)
	require.Equal(t, assetName, archiveName)
	require.Equal(t, "https://api.github.com/repos/lien0219/modurelay/releases/assets/123", downloadURL)
	require.Equal(t, "https://api.github.com/repos/lien0219/modurelay/releases/assets/124", checksumURL)
}

func TestUpdateServiceRejectsUnsignedReleaseAsset(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{},
		"0.3.1",
		"release",
		"binary",
	)

	_, _, _, err := svc.selectReleaseAssets([]Asset{
		{
			Name:        "modurelay_0.3.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz",
			DownloadURL: "https://api.github.com/repos/lien0219/modurelay/releases/assets/123",
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "checksums.txt is required")
}

func TestUpdateServiceDockerRollbackUsesOnlyProductionImageTags(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{containerTags: []string{
			"main", "sha-abcdef", "main-v0.3.1-abcdef", "main-v0.3.1",
			"main-v0.3.0", "main-v0.2.9", "main-v0.2.8", "main-v0.2.7",
			"0.2.6", "main-vbad",
		}},
		"main-v0.3.1-a1b2c3d4",
		"release",
		"docker",
	)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Equal(t, []RollbackVersion{
		{
			Version:       "0.3.0",
			HTMLURL:       ghcrPackageURL,
			Image:         "ghcr.io/lien0219/modurelay:main-v0.3.0",
			DeployCommand: "./deploy-main.sh ghcr.io/lien0219/modurelay:main-v0.3.0",
			Method:        RollbackMethodHostCommand,
		},
		{
			Version:       "0.2.9",
			HTMLURL:       ghcrPackageURL,
			Image:         "ghcr.io/lien0219/modurelay:main-v0.2.9",
			DeployCommand: "./deploy-main.sh ghcr.io/lien0219/modurelay:main-v0.2.9",
			Method:        RollbackMethodHostCommand,
		},
		{
			Version:       "0.2.8",
			HTMLURL:       ghcrPackageURL,
			Image:         "ghcr.io/lien0219/modurelay:main-v0.2.8",
			DeployCommand: "./deploy-main.sh ghcr.io/lien0219/modurelay:main-v0.2.8",
			Method:        RollbackMethodHostCommand,
		},
	}, versions)
}

func TestUpdateServiceDockerCheckUsesGHCRAndNormalizesEmbeddedVersion(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{containerTags: []string{
			"main-v0.3.2", "main-v0.3.1", "main-v0.3.0",
		}},
		"main-v0.3.1-a1b2c3d4",
		"release",
		"docker",
	)

	info, err := svc.CheckUpdate(context.Background(), true)

	require.NoError(t, err)
	require.True(t, info.HasUpdate)
	require.Equal(t, "0.3.2", info.LatestVersion)
	require.Equal(t, DeploymentModeDocker, info.DeploymentMode)
	require.Equal(t, "ghcr.io/lien0219/modurelay:main-v0.3.2", info.TargetImage)
	require.Equal(t, "./deploy-main.sh ghcr.io/lien0219/modurelay:main-v0.3.2", info.DeployCommand)
}

func TestUpdateServiceDockerRejectsInContainerRollback(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{},
		"main-v0.3.1-a1b2c3d4",
		"release",
		"docker",
	)

	err := svc.RollbackToVersion(context.Background(), "0.3.0")

	require.ErrorIs(t, err, ErrHostDeploymentRequired)
	require.ErrorIs(t, svc.PerformUpdate(context.Background()), ErrHostDeploymentRequired)
}

func TestUpdateServiceSourceBuildRejectsInPlaceMutation(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{},
		"0.3.1",
		"source",
		"source",
	)

	versions, err := svc.ListRollbackVersions(context.Background())
	require.NoError(t, err)
	require.Empty(t, versions)
	require.ErrorIs(t, svc.PerformUpdate(context.Background()), ErrInPlaceUpdateUnavailable)
	require.ErrorIs(t, svc.RollbackToVersion(context.Background(), "0.3.0"), ErrInPlaceUpdateUnavailable)
}

func TestNormalizeSemanticVersionFromDockerBuildIdentifier(t *testing.T) {
	require.Equal(t, "0.3.0", normalizeSemanticVersion("main-v0.3.0-81a2f23b25fb"))
	require.Equal(t, [3]int{0, 3, 0}, parseVersion("main-v0.3.0-81a2f23b25fb"))
}
