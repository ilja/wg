package initsetup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestInstallFollowsSourceSymlinkToRegularFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "template-target.sh")
	source := filepath.Join(root, "template.sh")
	destination := filepath.Join(root, "repo", ".config", "setup.sh")
	if err := os.WriteFile(target, []byte("linked template\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, source); err != nil {
		t.Fatal(err)
	}

	if err := Install(source, destination, false); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "linked template\n" {
		t.Fatalf("unexpected content %q", content)
	}
}

func TestInstallRefusesNonRegularSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "template")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "repo", ".config", "setup.sh")

	err := Install(source, destination, false)
	if err == nil || !strings.Contains(err.Error(), source) || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("expected source type diagnostic, got %v", err)
	}
	if _, err := os.Lstat(filepath.Dir(destination)); !os.IsNotExist(err) {
		t.Fatalf("destination parent should not be created, stat err: %v", err)
	}
}

func TestInstallRefusesUnsafeDestinationParent(t *testing.T) {
	for _, parentType := range []string{"file", "symlink"} {
		t.Run(parentType, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "template.sh")
			if err := os.WriteFile(source, []byte("template\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			parent := filepath.Join(root, "repo", ".config")
			if err := os.MkdirAll(filepath.Dir(parent), 0o755); err != nil {
				t.Fatal(err)
			}
			switch parentType {
			case "file":
				if err := os.WriteFile(parent, []byte("not a directory\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(root, "config-target")
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, parent); err != nil {
					t.Fatal(err)
				}
			}

			err := Install(source, filepath.Join(parent, "setup.sh"), false)
			if err == nil || !strings.Contains(err.Error(), parent) {
				t.Fatalf("expected parent type diagnostic, got %v", err)
			}
			if parentType == "symlink" {
				if _, err := os.Stat(filepath.Join(root, "config-target", "setup.sh")); !os.IsNotExist(err) {
					t.Fatalf("symlink target should remain untouched, stat err: %v", err)
				}
			}
		})
	}
}

func TestInstallNoClobberUnderConcurrentPublication(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "repo", ".config", "setup.sh")
	const contenders = 8
	sources := make([]string, contenders)
	for index := range sources {
		sources[index] = filepath.Join(root, fmt.Sprintf("template-%d.sh", index))
		if err := os.WriteFile(sources[index], []byte(fmt.Sprintf("template %d\n", index)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	start := make(chan struct{})
	errs := make([]error, contenders)
	var wait sync.WaitGroup
	for index := range sources {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			errs[index] = Install(sources[index], destination, false)
		}(index)
	}
	close(start)
	wait.Wait()

	winners := 0
	winnerContent := ""
	for index, err := range errs {
		if err == nil {
			winners++
			winnerContent = fmt.Sprintf("template %d\n", index)
		}
	}
	if winners != 1 {
		t.Fatalf("expected one successful publisher, got %d errors=%v", winners, errs)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != winnerContent {
		t.Fatalf("destination was clobbered: want %q, got %q", winnerContent, content)
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
