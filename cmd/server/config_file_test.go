package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeServerConfig(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestServerJSONConfig(t *testing.T) {
	unsetServerEnv(t)
	path := writeServerConfig(t, `{"address":"file:8081","restore":false,"store_interval":"500ms","store_file":"metrics.db","database_dsn":"dsn","crypto_key":"private.pem","key":"secret","audit_file":"audit.log","audit_url":"http://audit"}`)
	want := config{ServerAddress: "file:8081", Restore: false, StoreInterval: 500 * time.Millisecond, FileStoragePath: "metrics.db", DatabaseDSN: "dsn", CryptoKey: "private.pem", Key: "secret", AuditFile: "audit.log", AuditURL: "http://audit"}
	for _, flag := range []string{"-c", "-config"} {
		cfg, err := parseConfig([]string{flag, path})
		if err != nil || cfg != want {
			t.Fatalf("%s: got %+v, %v; want %+v", flag, cfg, err, want)
		}
	}
	t.Setenv("CONFIG", path)
	cfg, err := parseConfig([]string{"-c", "missing.json"})
	if err != nil || cfg != want {
		t.Fatalf("CONFIG: %+v, %v", cfg, err)
	}
}

func TestServerJSONPrecedence(t *testing.T) {
	unsetServerEnv(t)
	path := writeServerConfig(t, `{"address":"file","restore":true,"store_interval":"500ms","store_file":"metrics.db","database_dsn":"dsn","crypto_key":"private.pem","key":"secret","audit_file":"audit.log","audit_url":"http://audit"}`)
	args := []string{"-config", path, "-a", "flag", "-r=false", "-i", "0", "-f=", "-d=", "-crypto-key=", "-k=", "-audit-file=", "-audit-url="}
	want := config{ServerAddress: "flag"}
	cfg, err := parseConfig(args)
	if err != nil || cfg != want {
		t.Fatalf("flags: %+v, %v; want %+v", cfg, err, want)
	}
	for name, value := range map[string]string{"ADDRESS": "env", "RESTORE": "true", "STORE_INTERVAL": "2", "STORE_FILE": "env.db", "DATABASE_DSN": "env-dsn", "CRYPTO_KEY": "env.pem", "KEY": "env-key", "AUDIT_FILE": "env.log", "AUDIT_URL": "http://env"} {
		t.Setenv(name, value)
	}
	want = config{ServerAddress: "env", Restore: true, StoreInterval: 2 * time.Second, FileStoragePath: "env.db", DatabaseDSN: "env-dsn", CryptoKey: "env.pem", Key: "env-key", AuditFile: "env.log", AuditURL: "http://env"}
	cfg, err = parseConfig(args)
	if err != nil || cfg != want {
		t.Fatalf("env: %+v, %v; want %+v", cfg, err, want)
	}
	t.Setenv("FILE_STORAGE_PATH", "legacy.db")
	cfg, err = parseConfig(args)
	if err != nil || cfg.FileStoragePath != "legacy.db" {
		t.Fatalf("legacy env: %+v, %v", cfg, err)
	}
}

func TestServerJSONDefaultsAndErrors(t *testing.T) {
	unsetServerEnv(t)
	defaults, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfig([]string{"-c", writeServerConfig(t, `{}`)})
	if err != nil || cfg != defaults {
		t.Fatalf("defaults: %+v, %v", cfg, err)
	}
	cfg, err = parseConfig([]string{"-c", writeServerConfig(t, `{"restore":false,"store_interval":"0s"}`)})
	if err != nil || cfg.Restore || cfg.StoreInterval != 0 {
		t.Fatalf("zero values: %+v, %v", cfg, err)
	}
	for _, data := range []string{``, `null`, `[]`, `{`, `{} {}`, `{"store_interval":1}`, `{"store_interval":"bad"}`, `{"store_interval":"-1s"}`, `{"restore":"false"}`} {
		if _, err := parseConfig([]string{"-c", writeServerConfig(t, data)}); err == nil {
			t.Errorf("accepted %q", data)
		}
	}
	if _, err := parseConfig([]string{"-c", filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("accepted missing file")
	}
}
