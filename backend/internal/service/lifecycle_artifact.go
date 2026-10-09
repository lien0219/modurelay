package service

import (
	"bytes"
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

	"github.com/google/uuid"
)

const LifecycleMaxArtifactBytes int64 = 64 << 20
const lifecycleChunkBytes = 64 << 10

var lifecycleArtifactMagic = []byte("MRLEX01\n")

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
	if _, err = dst.Write(append(append([]byte{}, lifecycleArtifactMagic...), salt...)); err != nil {
		return err
	}
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
			nonce, aad := lifecycleChunkAuth(salt, scope, counter, uint32(n))
			sealed := aead.Seal(nil, nonce, buffer[:n], aad)
			if err = binary.Write(dst, binary.BigEndian, uint32(n)); err != nil {
				return err
			}
			if _, err = dst.Write(sealed); err != nil {
				return err
			}
			counter++
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	nonce, aad := lifecycleChunkAuth(salt, scope, counter, 0)
	if err = binary.Write(dst, binary.BigEndian, uint32(0)); err != nil {
		return err
	}
	_, err = dst.Write(aead.Seal(nil, nonce, nil, aad))
	return err
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
	var total int64
	var counter uint64
	for {
		var length uint32
		if err = binary.Read(src, binary.BigEndian, &length); err != nil {
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
		if _, err = io.ReadFull(src, sealed); err != nil {
			return err
		}
		nonce, aad := lifecycleChunkAuth(salt, scope, counter, length)
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
			return nil
		}
		if _, err = dst.Write(plain); err != nil {
			return err
		}
		counter++
	}
}
