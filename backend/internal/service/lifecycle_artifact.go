package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
)

const LifecycleMaxArtifactBytes int64 = 64 << 20
const lifecycleChunkBytes = 64 << 10

var lifecycleArtifactMagic = []byte("MRLEX01\n")
var lifecycleArtifactMagicV2 = []byte("MRLEX02\n")

func LifecycleObjectKey(workspaceID int64, jobID, attemptID string) (string, error) {
	if workspaceID <= 0 || !validLifecycleUUID(jobID) || !validLifecycleUUID(attemptID) {
		return "", ErrWorkspaceInvalid
	}
	return fmt.Sprintf("workspace-exports/%d/%s/%s.enc", workspaceID, jobID, attemptID), nil
}

func validLifecycleUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func ValidLifecycleObjectKey(key string, workspaceID int64, jobID string) bool {
	prefix := "workspace-exports/" + strconv.FormatInt(workspaceID, 10) + "/" + jobID + "/"
	if workspaceID <= 0 || !validLifecycleUUID(jobID) || !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, ".enc") {
		return false
	}
	attempt := strings.TrimSuffix(strings.TrimPrefix(key, prefix), ".enc")
	return validLifecycleUUID(attempt)
}

// A random per-artifact salt derives an independent AES key. Every chunk and
// the mandatory terminal chunk are authenticated with scope, salt and counter.
func lifecycleArtifactCipher(key, salt []byte, scope string) (cipher.AEAD, error) {
	if len(key) != 32 || len(salt) != 32 || scope == "" {
		return nil, errors.New("invalid lifecycle encryption configuration")
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("modurelay-lifecycle-v1\x00" + scope + "\x00"))
	_, _ = mac.Write(salt)
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func lifecycleChunkAuth(salt []byte, scope string, counter uint64, length uint32) ([]byte, []byte) {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[4:], counter)
	aad := append([]byte("MRLEX01\x00"+scope+"\x00"), salt...)
	aad = append(aad, nonce...)
	aad = binary.BigEndian.AppendUint32(aad, length)
	return nonce, aad
}

func EncryptLifecycleArtifact(dst io.Writer, src io.Reader, key []byte, scope string) error {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	aead, err := lifecycleArtifactCipher(key, salt, scope)
	if err != nil {
		return err
	}
	if err = writeLifecycleBytes(dst, append(append([]byte{}, lifecycleArtifactMagic...), salt...)); err != nil {
		return err
	}
	return encryptLifecycleChunks(dst, src, aead, salt, scope, nil)
}

func EncryptLifecycleArtifactV2(ctx context.Context, dst io.Writer, src io.Reader, ring *LifecycleKeyRing, scope string) error {
	if ring == nil || ring.Resolver == nil || !config.ValidLifecycleKeyIdentifier(ring.ActiveID) || ring.ActiveVersion == 0 {
		return ErrLifecycleKeyUnavailable
	}
	key, err := ring.Resolver.ResolveLifecycleKey(ctx, ring.ActiveID, ring.ActiveVersion)
	if err != nil {
		return err
	}
	salt := make([]byte, 32)
	if _, err = rand.Read(salt); err != nil {
		return err
	}
	header := append(append([]byte{}, lifecycleArtifactMagicV2...), byte(len(ring.ActiveID)))
	header = append(header, []byte(ring.ActiveID)...)
	header = binary.BigEndian.AppendUint32(header, ring.ActiveVersion)
	header = append(header, salt...)
	aead, err := lifecycleV2Cipher(key, header, scope)
	if err != nil {
		return err
	}
	if err = writeLifecycleBytes(dst, header); err != nil {
		return err
	}
	return encryptLifecycleChunks(dst, src, aead, salt, scope, header)
}

func lifecycleV2Cipher(key, header []byte, scope string) (cipher.AEAD, error) {
	if len(key) != 32 || scope == "" {
		return nil, ErrLifecycleKeyUnavailable
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("modurelay-lifecycle-v2\x00" + scope + "\x00"))
	_, _ = mac.Write(header)
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func lifecycleArtifactChunkAuth(salt []byte, scope string, counter uint64, length uint32, v2Header []byte) ([]byte, []byte) {
	if v2Header == nil {
		return lifecycleChunkAuth(salt, scope, counter, length)
	}
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[4:], counter)
	aad := append([]byte("MRLEX02\x00"+scope+"\x00"), v2Header...)
	aad = append(aad, nonce...)
	return nonce, binary.BigEndian.AppendUint32(aad, length)
}

func writeLifecycleBytes(dst io.Writer, data []byte) error {
	n, err := dst.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

func encryptLifecycleChunks(dst io.Writer, src io.Reader, aead cipher.AEAD, salt []byte, scope string, v2Header []byte) error {
	input := io.LimitReader(src, LifecycleMaxArtifactBytes+1)
	buffer := make([]byte, lifecycleChunkBytes)
	var total int64
	var counter uint64
	for {
		n, readErr := io.ReadFull(input, buffer)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return readErr
		}
		if n > 0 {
			total += int64(n)
			if total > LifecycleMaxArtifactBytes {
				return ErrLifecycleExportLimit
			}
			nonce, aad := lifecycleArtifactChunkAuth(salt, scope, counter, uint32(n), v2Header)
			sealed := aead.Seal(nil, nonce, buffer[:n], aad)
			if err := writeLifecycleBytes(dst, binary.BigEndian.AppendUint32(nil, uint32(n))); err != nil {
				return err
			}
			if err := writeLifecycleBytes(dst, sealed); err != nil {
				return err
			}
			counter++
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	nonce, aad := lifecycleArtifactChunkAuth(salt, scope, counter, 0, v2Header)
	if err := writeLifecycleBytes(dst, []byte{0, 0, 0, 0}); err != nil {
		return err
	}
	return writeLifecycleBytes(dst, aead.Seal(nil, nonce, nil, aad))
}

func DecryptLifecycleArtifact(dst io.Writer, src io.Reader, key []byte, scope string) error {
	header := make([]byte, len(lifecycleArtifactMagic)+32)
	if _, err := io.ReadFull(src, header); err != nil {
		return err
	}
	if !bytes.Equal(header[:len(lifecycleArtifactMagic)], lifecycleArtifactMagic) {
		return errors.New("invalid lifecycle artifact format")
	}
	salt := header[len(lifecycleArtifactMagic):]
	aead, err := lifecycleArtifactCipher(key, salt, scope)
	if err != nil {
		return err
	}
	return decryptLifecycleChunks(dst, src, aead, salt, scope, nil)
}

func DecryptLifecycleArtifactWithKeys(ctx context.Context, dst io.Writer, src io.Reader, ring *LifecycleKeyRing, scope string) error {
	if ring == nil || ring.Resolver == nil {
		return ErrLifecycleKeyUnavailable
	}
	magic := make([]byte, len(lifecycleArtifactMagic))
	if _, err := io.ReadFull(src, magic); err != nil {
		return err
	}
	if bytes.Equal(magic, lifecycleArtifactMagic) {
		salt := make([]byte, 32)
		if _, err := io.ReadFull(src, salt); err != nil {
			return err
		}
		key, err := ring.Resolver.LegacyLifecycleKey(ctx)
		if err != nil {
			return err
		}
		aead, err := lifecycleArtifactCipher(key, salt, scope)
		if err != nil {
			return err
		}
		return decryptLifecycleChunks(dst, src, aead, salt, scope, nil)
	}
	if !bytes.Equal(magic, lifecycleArtifactMagicV2) {
		return errors.New("invalid lifecycle artifact format")
	}
	var size [1]byte
	if _, err := io.ReadFull(src, size[:]); err != nil {
		return err
	}
	if size[0] == 0 || size[0] > 64 {
		return errors.New("invalid lifecycle artifact key ID")
	}
	fields := make([]byte, int(size[0])+4+32)
	if _, err := io.ReadFull(src, fields); err != nil {
		return err
	}
	id := string(fields[:size[0]])
	version := binary.BigEndian.Uint32(fields[size[0] : int(size[0])+4])
	if !config.ValidLifecycleKeyIdentifier(id) || version == 0 {
		return ErrLifecycleKeyUnavailable
	}
	key, err := ring.Resolver.ResolveLifecycleKey(ctx, id, version)
	if err != nil {
		return err
	}
	header := append(append(magic, size[0]), fields...)
	aead, err := lifecycleV2Cipher(key, header, scope)
	if err != nil {
		return err
	}
	return decryptLifecycleChunks(dst, src, aead, fields[int(size[0])+4:], scope, header)
}

func decryptLifecycleChunks(dst io.Writer, src io.Reader, aead cipher.AEAD, salt []byte, scope string, v2Header []byte) error {
	// Even valid early chunks are withheld until terminal authentication and
	// EOF succeed. Callers cannot accidentally expose a truncated archive.
	var validated bytes.Buffer
	var total int64
	var counter uint64
	for {
		var length uint32
		if err := binary.Read(src, binary.BigEndian, &length); err != nil {
			return err
		}
		if length > lifecycleChunkBytes {
			return ErrLifecycleExportLimit
		}
		total += int64(length)
		if total > LifecycleMaxArtifactBytes {
			return ErrLifecycleExportLimit
		}
		sealed := make([]byte, int(length)+aead.Overhead())
		if _, err := io.ReadFull(src, sealed); err != nil {
			return err
		}
		nonce, aad := lifecycleArtifactChunkAuth(salt, scope, counter, length, v2Header)
		plain, openErr := aead.Open(nil, nonce, sealed, aad)
		if openErr != nil {
			return errors.New("lifecycle artifact authentication failed")
		}
		if length == 0 {
			var extra [1]byte
			n, readErr := src.Read(extra[:])
			if n != 0 || readErr != io.EOF {
				return errors.New("lifecycle artifact has trailing data")
			}
			_, err := io.Copy(dst, &validated)
			return err
		}
		_, _ = validated.Write(plain)
		counter++
	}
}
