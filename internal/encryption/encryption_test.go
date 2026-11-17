package encryption

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvelope(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("metrics"), 10000)
	encrypted, err := Encrypt(&key.PublicKey, payload)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := Decrypt(key, encrypted)
	if err != nil || !bytes.Equal(payload, decrypted) {
		t.Fatalf("roundtrip: %v", err)
	}
	second, err := Encrypt(&key.PublicKey, payload)
	if err != nil || bytes.Equal(encrypted, second) {
		t.Fatalf("randomization: %v", err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(other, encrypted); err == nil {
		t.Fatal("accepted wrong key")
	}
	for _, index := range []int{0, key.Size(), len(encrypted) - 1} {
		corrupted := bytes.Clone(encrypted)
		corrupted[index] ^= 1
		if _, err := Decrypt(key, corrupted); err == nil {
			t.Fatalf("accepted corruption at %d", index)
		}
	}
	for _, n := range []int{0, 1, key.Size(), len(encrypted) - 1} {
		if _, err := Decrypt(key, encrypted[:n]); err == nil {
			t.Fatalf("accepted truncated length %d", n)
		}
	}
}

func TestLoadKeys(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		data    []byte
		private bool
	}{
		{"PUBLIC KEY", pub, false}, {"RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(&key.PublicKey), false},
		{"PRIVATE KEY", priv, true}, {"RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "key.pem")
			if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: tc.name, Bytes: tc.data}), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.private {
				loaded, err := LoadPrivateKey(path)
				if err != nil || !loaded.Equal(key) {
					t.Fatalf("load: %v", err)
				}
				if _, err := LoadPublicKey(path); err == nil {
					t.Fatal("accepted private key as public")
				}
			} else {
				loaded, err := LoadPublicKey(path)
				if err != nil || !loaded.Equal(&key.PublicKey) {
					t.Fatalf("load: %v", err)
				}
				if _, err := LoadPrivateKey(path); err == nil {
					t.Fatal("accepted public key as private")
				}
			}
		})
	}
	for _, loader := range []func(string) error{
		func(p string) error { _, err := LoadPublicKey(p); return err },
		func(p string) error { _, err := LoadPrivateKey(p); return err },
	} {
		if err := loader(""); err != nil {
			t.Fatal(err)
		}
		if err := loader(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Fatal("accepted missing file")
		}
		path := filepath.Join(t.TempDir(), "bad.pem")
		if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := loader(path); err == nil {
			t.Fatal("accepted invalid PEM")
		}
	}
}
