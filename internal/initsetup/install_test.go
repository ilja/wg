package initsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCreatesExecutableHook(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "template.sh")
	content := "#!/bin/sh\nprintf 'ready\\n'\n"
	if err := os.WriteFile(source, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "repo", ".config", "setup.sh")

	if err := Install(source, destination, false); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("content mismatch: want %q, got %q", content, got)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o740); got != want {
		t.Fatalf("mode mismatch: want %o, got %o", want, got)
	}
	entries, err := os.ReadDir(filepath.Dir(destination))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".setup.sh-") {
			t.Fatalf("temporary file was not cleaned up: %s", entry.Name())
		}
	}
}
