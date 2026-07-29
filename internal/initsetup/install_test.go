package initsetup

import (
	"fmt"
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

func TestInstallRefusesExistingHookWithoutForce(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "template.sh")
	destination := filepath.Join(root, "repo", ".config", "setup.sh")
	if err := os.WriteFile(source, []byte("new content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("hand edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Install(source, destination, false)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected actionable force error, got %v", err)
	}
	got, readErr := os.ReadFile(destination)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "hand edited\n" {
		t.Fatalf("existing hook changed: got %q", got)
	}
}

func TestInstallForceReplacesRegularHook(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "template.sh")
	destination := filepath.Join(root, "repo", ".config", "setup.sh")
	content := "#!/bin/sh\nprintf 'replacement\\n'\n"
	if err := os.WriteFile(source, []byte(content), 0o664); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(source, 0o664); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old content\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := Install(source, destination, true); err != nil {
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
	if got, want := info.Mode().Perm(), os.FileMode(0o764); got != want {
		t.Fatalf("mode mismatch: want %o, got %o", want, got)
	}
}

func TestInstallRefusesUnsafeDestination(t *testing.T) {
	for _, force := range []bool{false, true} {
		force := force
		t.Run(fmt.Sprintf("force=%t", force), func(t *testing.T) {
			for _, destinationType := range []string{"directory", "symlink"} {
				destinationType := destinationType
				t.Run(destinationType, func(t *testing.T) {
					root := t.TempDir()
					source := filepath.Join(root, "template.sh")
					destination := filepath.Join(root, "repo", ".config", "setup.sh")
					if err := os.WriteFile(source, []byte("template\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
						t.Fatal(err)
					}
					switch destinationType {
					case "directory":
						if err := os.Mkdir(destination, 0o755); err != nil {
							t.Fatal(err)
						}
					case "symlink":
						target := filepath.Join(root, "target.sh")
						if err := os.WriteFile(target, []byte("target\n"), 0o600); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(target, destination); err != nil {
							t.Fatal(err)
						}
					}

					if err := Install(source, destination, force); err == nil {
						t.Fatal("expected unsafe destination error")
					}
					info, err := os.Lstat(destination)
					if err != nil {
						t.Fatal(err)
					}
					if destinationType == "directory" && !info.IsDir() {
						t.Fatalf("destination is no longer a directory: %v", info.Mode())
					}
					if destinationType == "symlink" && info.Mode()&os.ModeSymlink == 0 {
						t.Fatalf("destination is no longer a symlink: %v", info.Mode())
					}
				})
			}
		})
	}
}
