package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
)

var ErrLifecycleKeyUnavailable = errors.New("lifecycle artifact key unavailable")
var ErrLifecycleKeyRingNotReady = errors.New("lifecycle key ring readers not ready")

// A trusted KMS/Secret Manager can implement this exact lookup boundary. The
// configured implementation is local secret custody, not a cloud KMS claim.
type LifecycleKeyResolver interface {
	LegacyLifecycleKey(context.Context) ([]byte, error)
	ResolveLifecycleKey(context.Context, string, uint32) ([]byte, error)
}

type lifecycleResolvedKey struct {
	version uint32
	key     []byte
}
type configuredLifecycleKeys struct {
	legacy []byte
	keys   map[string]lifecycleResolvedKey
}

func (r *configuredLifecycleKeys) LegacyLifecycleKey(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(r.legacy) != 32 {
		return nil, ErrLifecycleKeyUnavailable
	}
	return append([]byte(nil), r.legacy...), nil
}
func (r *configuredLifecycleKeys) ResolveLifecycleKey(ctx context.Context, id string, version uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entry, ok := r.keys[id]
	if !ok || version != entry.version {
		return nil, ErrLifecycleKeyUnavailable
	}
	return append([]byte(nil), entry.key...), nil
}

type LifecycleKeyRing struct {
	Resolver      LifecycleKeyResolver
	ActiveID      string
	ActiveVersion uint32
	Fingerprint   string
}

func NewLifecycleKeyRing(c config.DataLifecycleConfig) (*LifecycleKeyRing, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	legacy, err := hex.DecodeString(c.EncryptionKey)
	if err != nil || len(legacy) != 32 {
		return nil, ErrLifecycleKeyUnavailable
	}
	resolver := &configuredLifecycleKeys{legacy: legacy, keys: make(map[string]lifecycleResolvedKey)}
	ring := &LifecycleKeyRing{Resolver: resolver, ActiveID: c.ActiveKeyID}
	entries := append([]config.LifecycleEncryptionKey(nil), c.Keys...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	h := sha256.New()
	_, _ = h.Write([]byte("modurelay-export-readers-v2\x00"))
	_, _ = h.Write(legacy)
	for _, entry := range entries {
		key, decodeErr := hex.DecodeString(entry.Key)
		if decodeErr != nil || len(key) != 32 || !config.ValidLifecycleKeyIdentifier(entry.ID) || entry.Version == 0 {
			return nil, ErrLifecycleKeyUnavailable
		}
		resolver.keys[entry.ID] = lifecycleResolvedKey{version: entry.Version, key: key}
		if entry.ID == c.ActiveKeyID {
			ring.ActiveVersion = entry.Version
		}
		_, _ = fmt.Fprintf(h, "\x00%s\x00%d\x00%s\x00", entry.ID, entry.Version, entry.Status)
		_, _ = h.Write(key)
	}
	ring.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return ring, nil
}

// No key bytes or secret configuration crosses this database port.
type LifecycleCryptoReader struct {
	InstanceID          string
	Token               string
	Fingerprint         string
	ExpectedInstanceIDs []string
	V2Readable          bool
	RollbackCompatible  bool
}

func (r LifecycleCryptoReader) InventoryFingerprint() string {
	ids := append([]string(nil), r.ExpectedInstanceIDs...)
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(strings.Join(ids, "\x00")))
	return hex.EncodeToString(sum[:])
}

type LifecycleCryptoRepository interface {
	RegisterLifecycleCryptoReader(context.Context, LifecycleCryptoReader, bool) error
	ClaimLifecycleExportWithCrypto(context.Context, LifecycleCryptoReader, bool) (*LifecycleExportJob, error)
}

func ValidLifecycleCryptoToken(token string) bool {
	id, err := uuid.Parse(token)
	return err == nil && id != uuid.Nil && id.String() == token
}
