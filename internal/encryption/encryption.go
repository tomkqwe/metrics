// Package encryption implements RSA-OAEP/AES-GCM envelopes for metric requests.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// Header identifies the encrypted request envelope.
const Header = "X-Metrics-Encryption"
const Scheme = "rsa-oaep-aes-256-gcm-v1"

// LoadPublicKey reads a PEM PKIX or PKCS#1 RSA public key. An empty path disables encryption.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	if path == "" {
		return nil, nil
	}
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	var key *rsa.PublicKey
	switch block.Type {
	case "RSA PUBLIC KEY":
		key, err = x509.ParsePKCS1PublicKey(block.Bytes)
	case "PUBLIC KEY":
		var parsed any
		parsed, err = x509.ParsePKIXPublicKey(block.Bytes)
		if err == nil {
			var ok bool
			key, ok = parsed.(*rsa.PublicKey)
			if !ok {
				err = errors.New("public key is not RSA")
			}
		}
	default:
		err = errors.New("expected RSA public key PEM")
	}
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	if key.N.BitLen() < 2048 {
		return nil, errors.New("RSA key must be at least 2048 bits")
	}
	return key, nil
}

// LoadPrivateKey reads an unencrypted PEM PKCS#1 or PKCS#8 RSA private key.
// An empty path disables decryption.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	if path == "" {
		return nil, nil
	}
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	var key *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			var ok bool
			key, ok = parsed.(*rsa.PrivateKey)
			if !ok {
				err = errors.New("private key is not RSA")
			}
		}
	default:
		err = errors.New("expected RSA private key PEM")
	}
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("validate private key: %w", err)
	}
	if key.N.BitLen() < 2048 {
		return nil, errors.New("RSA key must be at least 2048 bits")
	}
	return key, nil
}

func readPEM(path string) (*pem.Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read crypto key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("crypto key is not PEM encoded")
	}
	return block, nil
}

// Encrypt returns wrapped AES key || nonce || authenticated ciphertext.
// Each message uses a fresh AES-256 key and nonce; RSA uses OAEP with SHA-256.
func Encrypt(key *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	if key == nil {
		return nil, errors.New("missing public key")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, key, secret, []byte(Scheme))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	envelope := append(wrapped, nonce...)
	return gcm.Seal(envelope, nonce, plaintext, []byte(Scheme)), nil
}

// Decrypt authenticates and opens an envelope. Invalid messages return an error.
func Decrypt(key *rsa.PrivateKey, envelope []byte) ([]byte, error) {
	if key == nil {
		return nil, errors.New("missing private key")
	}
	size := key.Size()
	if len(envelope) < size+12+16 {
		return nil, errors.New("truncated encrypted message")
	}
	secret, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, key, envelope[:size], []byte(Scheme))
	if err != nil {
		return nil, err
	}
	if len(secret) != 32 {
		return nil, errors.New("invalid AES key length")
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, envelope[size:size+gcm.NonceSize()], envelope[size+gcm.NonceSize():], []byte(Scheme))
}
