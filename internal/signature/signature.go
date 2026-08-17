package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const Header = "HashSHA256"

func Calculate(value []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(value)

	return hex.EncodeToString(mac.Sum(nil))
}

func Verify(value []byte, key, hash string) bool {
	got, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(value)

	return hmac.Equal(got, mac.Sum(nil))
}
