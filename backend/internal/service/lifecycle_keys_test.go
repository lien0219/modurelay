package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func fixtureLifecycleKeyRing(t *testing.T) *LifecycleKeyRing {
	t.Helper()
	ring, err := NewLifecycleKeyRing(config.DataLifecycleConfig{Enabled: true, EncryptionKey: strings.Repeat("07", 32), Bucket: "fixture", AccessKeyID: "fixture", SecretAccessKey: "fixture", ActiveKeyID: "key-new", Keys: []config.LifecycleEncryptionKey{
		{ID: "key-old", Version: 1, Status: "decrypt_only", Key: strings.Repeat("08", 32)},
		{ID: "key-new", Version: 2, Status: "active", Key: strings.Repeat("09", 32)},
	}})
	require.NoError(t, err)
	return ring
}

func TestLifecycleKeyRingReadsFrozenV1AndMixedVersions(t *testing.T) {
	// Independent AES-GCM fixture using the documented original Phase G layout,
	// fixed synthetic salt/key, NOT produced by the new encoder under test.
	frozen, err := base64.StdEncoding.DecodeString("TVJMRVgwMQoAAQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHwAAAB8g5cgnoFHWzYoTl+ls/fMzZIZCHjLB6Sv5M3jKZCzQEUS6EHx0NWaUlEvzNCp29gAAAAAKqxU/0kL/FG4z5QRfRPyR")
	require.NoError(t, err)
	ring := fixtureLifecycleKeyRing(t)
	var decoded bytes.Buffer
	require.NoError(t, DecryptLifecycleArtifactWithKeys(context.Background(), &decoded, bytes.NewReader(frozen), ring, "42/job/attempt"))
	require.Equal(t, "Phase G retained export fixture", decoded.String())
	for _, version := range []int{1, 2} {
		var sealed, out bytes.Buffer
		if version == 1 {
			err = EncryptLifecycleArtifact(&sealed, strings.NewReader("mixed"), bytes.Repeat([]byte{7}, 32), "42/job/attempt")
		} else {
			err = EncryptLifecycleArtifactV2(context.Background(), &sealed, strings.NewReader("mixed"), ring, "42/job/attempt")
		}
		require.NoError(t, err)
		require.NoError(t, DecryptLifecycleArtifactWithKeys(context.Background(), &out, bytes.NewReader(sealed.Bytes()), ring, "42/job/attempt"))
		require.Equal(t, "mixed", out.String())
		if version == 1 {
			out.Reset()
			require.NoError(t, DecryptLifecycleArtifact(&out, bytes.NewReader(sealed.Bytes()), bytes.Repeat([]byte{7}, 32), "42/job/attempt"), "old binary compatibility")
		}
	}
}

func TestLifecycleKeyRingRotationPreservesHistoricalV2(t *testing.T) {
	ring := fixtureLifecycleKeyRing(t)
	var sealed bytes.Buffer
	require.NoError(t, EncryptLifecycleArtifactV2(context.Background(), &sealed, strings.NewReader("retained"), ring, "scope"))
	resolver := ring.Resolver
	ring.ActiveID = "key-old"
	ring.ActiveVersion = 1 // Compatible reader keeps historical explicit mapping.
	var out bytes.Buffer
	require.NoError(t, DecryptLifecycleArtifactWithKeys(context.Background(), &out, bytes.NewReader(sealed.Bytes()), ring, "scope"))
	require.Equal(t, "retained", out.String())
	require.Same(t, resolver, ring.Resolver)
	var next bytes.Buffer
	require.NoError(t, EncryptLifecycleArtifactV2(context.Background(), &next, strings.NewReader("next"), ring, "scope"))
	out.Reset()
	require.NoError(t, DecryptLifecycleArtifactWithKeys(context.Background(), &out, bytes.NewReader(next.Bytes()), fixtureLifecycleKeyRing(t), "scope"))
	require.Equal(t, "next", out.String())
}

func TestLifecycleKeyRingTamperingNeverReleasesPlaintext(t *testing.T) {
	ring := fixtureLifecycleKeyRing(t)
	plain := bytes.Repeat([]byte("financial evidence\n"), 10000)
	var sealed, other bytes.Buffer
	require.NoError(t, EncryptLifecycleArtifactV2(context.Background(), &sealed, bytes.NewReader(plain), ring, "scope"))
	require.NoError(t, EncryptLifecycleArtifactV2(context.Background(), &other, bytes.NewReader(plain), ring, "scope"))
	require.NotEqual(t, sealed.Bytes(), other.Bytes(), "each object has independent random salt")
	for _, position := range []int{0, 8, 9, 17, 25, sealed.Len() - 1} {
		data := append([]byte{}, sealed.Bytes()...)
		data[position] ^= 0x80
		var out bytes.Buffer
		require.Error(t, DecryptLifecycleArtifactWithKeys(context.Background(), &out, bytes.NewReader(data), ring, "scope"))
		require.Empty(t, out.Bytes())
	}
	for _, data := range [][]byte{sealed.Bytes()[:sealed.Len()-1], sealed.Bytes()[:12], append(append([]byte{}, sealed.Bytes()...), 1)} {
		var out bytes.Buffer
		require.Error(t, DecryptLifecycleArtifactWithKeys(context.Background(), &out, bytes.NewReader(data), ring, "scope"))
		require.Empty(t, out.Bytes())
	}
	var out bytes.Buffer
	require.Error(t, DecryptLifecycleArtifactWithKeys(context.Background(), &out, bytes.NewReader(sealed.Bytes()), ring, "foreign-workspace/object"))
	require.Empty(t, out.Bytes())
	for _, change := range []string{"missing", "wrong", "version"} {
		r := fixtureLifecycleKeyRing(t)
		keys, ok := r.Resolver.(*configuredLifecycleKeys)
		require.True(t, ok)
		switch change {
		case "missing":
			delete(keys.keys, "key-new")
		case "wrong":
			keys.keys["key-new"] = lifecycleResolvedKey{version: 2, key: bytes.Repeat([]byte{10}, 32)}
		case "version":
			keys.keys["key-new"] = lifecycleResolvedKey{version: 3, key: bytes.Repeat([]byte{9}, 32)}
		}
		out.Reset()
		require.Error(t, DecryptLifecycleArtifactWithKeys(context.Background(), &out, bytes.NewReader(sealed.Bytes()), r, "scope"))
		require.Empty(t, out.Bytes())
	}
}

func TestLifecycleKeyRingFingerprintIncludesHistoricalBytesVersionsAndStatus(t *testing.T) {
	a := fixtureLifecycleKeyRing(t)
	b := fixtureLifecycleKeyRing(t)
	require.Equal(t, a.Fingerprint, b.Fingerprint)
	c := config.DataLifecycleConfig{Enabled: true, EncryptionKey: strings.Repeat("07", 32), Bucket: "fixture", AccessKeyID: "fixture", SecretAccessKey: "fixture", ActiveKeyID: "key-new", Keys: []config.LifecycleEncryptionKey{{ID: "key-old", Version: 1, Status: "decrypt_only", Key: strings.Repeat("10", 32)}, {ID: "key-new", Version: 2, Status: "active", Key: strings.Repeat("09", 32)}}}
	b, err := NewLifecycleKeyRing(c)
	require.NoError(t, err)
	require.NotEqual(t, a.Fingerprint, b.Fingerprint)
}

func TestLifecycleAdminDiagnosticsDescribeConfiguredKeyRingWithoutCertifyingCustody(t *testing.T) {
	s := NewWorkspaceService(&lifecycleRuntimeRepo{})
	s.lifecycle = &LifecycleRuntime{cfg: config.DataLifecycleConfig{Enabled: true}, keys: fixtureLifecycleKeyRing(t), key: bytes.Repeat([]byte{7}, 32)}
	v := AdminExportRotation{Format: "MRLEX01", SingleKeyNoID: true}
	s.appendAdminRotationCapabilities(&v)
	require.Equal(t, "MRLEX01/MRLEX02", v.Format)
	require.False(t, v.SingleKeyNoID)
	require.False(t, v.RotationCertified)
	require.Equal(t, "blocked", v.State)
}
