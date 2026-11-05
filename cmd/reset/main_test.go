package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func testModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module resettest\n\ngo 1.26.4\n")
	return root
}

func TestGenerate(t *testing.T) {
	root := testModule(t)
	for _, name := range []string{"types.go", "types_test.go"} {
		data, err := os.ReadFile(filepath.Join("testdata", "sample", name))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(root, "nested", "sample", name), string(data))
	}
	writeTestFile(t, filepath.Join(root, "root.go"), "package root\n// generate:reset\ntype Root struct { Name string }\n")
	writeTestFile(t, filepath.Join(root, "plain", "plain.go"), "package plain\ntype Plain struct{}\n")
	if err := generate(root); err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(root, "nested", "sample", "reset.gen.go")
	first, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, []byte("*Ignored")) {
		t.Fatal("unmarked struct received a method")
	}
	if _, err := os.Stat(filepath.Join(root, "reset.gen.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "plain", "reset.gen.go")); !os.IsNotExist(err) {
		t.Fatal("generated output for unmarked package")
	}
	if err := generate(root); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("generation is not deterministic")
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code tests: %v\n%s", err, out)
	}
}

func TestGenerateRejectsInvalidTargets(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"nonstruct", "// generate:reset\ntype Value int", "requires a defined struct"},
		{"alias", "// generate:reset\ntype Value = struct{}", "requires a defined struct"},
		{"existing method", "// generate:reset\ntype Value struct{}\nfunc (*Value) Reset() {}", "already has a Reset method"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := testModule(t)
			writeTestFile(t, filepath.Join(root, "input.go"), "package sample\n"+tc.source+"\n")
			err := generate(root)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestGenerateProtectsHandwrittenFile(t *testing.T) {
	root := testModule(t)
	writeTestFile(t, filepath.Join(root, "input.go"), "package sample\n// generate:reset\ntype Value struct{}\n")
	path := filepath.Join(root, "reset.gen.go")
	original := "package sample\n// handwritten\n"
	writeTestFile(t, path, original)
	if err := generate(root); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatal("handwritten file was changed")
	}
}
