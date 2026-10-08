// Package signature calculates and verifies HTTP payload HMAC-SHA256 signatures.
package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Header is the HTTP header carrying a hexadecimal HMAC-SHA256 signature.
const Header = "HashSHA256"

// Calculate returns the hexadecimal HMAC-SHA256 of value using key.
func Calculate(value []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(value)

	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a hexadecimal HMAC-SHA256 signature using a constant-time MAC comparison.
func Verify(value []byte, key, hash string) bool {
	got, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(value)

	return hmac.Equal(got, mac.Sum(nil))
}
