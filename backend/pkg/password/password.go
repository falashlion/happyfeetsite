// Package password provides secure password hashing using PBKDF2-SHA256
// with a random 32-byte salt. No external dependencies — stdlib only.
// Format: "pbkdf2$<iterations>$<hex-salt>$<hex-hash>"
package password

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	iterations = 310000 // NIST recommendation for PBKDF2-SHA256
	saltLen    = 32
	keyLen     = 32
)

var ErrMismatch = errors.New("password: mismatch")

// Hash hashes a plaintext password and returns the encoded hash string.
func Hash(plaintext string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password: generate salt: %w", err)
	}
	dk := pbkdf2Key([]byte(plaintext), salt, iterations, keyLen)
	return fmt.Sprintf("pbkdf2$%d$%s$%s", iterations, hex.EncodeToString(salt), hex.EncodeToString(dk)), nil
}

// Verify checks a plaintext password against an encoded hash.
func Verify(plaintext, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2" {
		return fmt.Errorf("password: invalid format")
	}
	iters, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("password: invalid iterations")
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("password: invalid salt")
	}
	expected, err := hex.DecodeString(parts[3])
	if err != nil {
		return fmt.Errorf("password: invalid hash")
	}
	actual := pbkdf2Key([]byte(plaintext), salt, iters, len(expected))
	if !hmac.Equal(actual, expected) {
		return ErrMismatch
	}
	return nil
}

// pbkdf2Key implements PBKDF2 with HMAC-SHA256.
func pbkdf2Key(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen

	var buf [4]byte
	dk := make([]byte, 0, numBlocks*hashLen)
	U := make([]byte, hashLen)

	for block := 1; block <= numBlocks; block++ {
		// U1 = PRF(password, salt || INT(block))
		prf.Reset()
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf[:4])
		T := prf.Sum(nil)
		copy(U, T)

		// U2..Uc
		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(U)
			prf.Sum(U[:0])
			for x := range T {
				T[x] ^= U[x]
			}
		}
		dk = append(dk, T...)
	}
	return dk[:keyLen]
}
