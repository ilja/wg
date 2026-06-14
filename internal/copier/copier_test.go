package copier

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileCreatesParentsCopiesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "nested", "dest.txt")
	if err := os.WriteFile(src, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst, 0o640); err != nil {
		t.Fatal(err)
	}

	assertCopiedFile(t, dst, "hello\n", 0o640)
}

func TestCopyFileWithCloneFallsBackWhenUnsupported(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "nested", "dest.txt")
	if err := os.WriteFile(src, []byte("fallback\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copyFileWithClone(src, dst, 0o644, func(string, string) error {
		return ErrCloneUnsupported
	}); err != nil {
		t.Fatal(err)
	}

	assertCopiedFile(t, dst, "fallback\n", 0o644)
}

func TestCopyFileWithCloneReturnsNonFallbackCloneErrors(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(src, []byte("source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("clone failed")

	err := copyFileWithClone(src, dst, 0o644, func(string, string) error {
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected clone error %v, got %v", boom, err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("destination should not exist after non-fallback error, stat err=%v", err)
	}
}

func TestFallbackOutputMatchesSuccessfulClonePath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(src, []byte("same\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fallbackDst := filepath.Join(dir, "fallback", "dest.txt")
	cloneDst := filepath.Join(dir, "clone", "dest.txt")

	if err := copyFileWithClone(src, fallbackDst, 0o640, func(string, string) error {
		return ErrCloneUnsupported
	}); err != nil {
		t.Fatal(err)
	}
	if err := copyFileWithClone(src, cloneDst, 0o640, func(src string, dst string) error {
		content, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, content, 0o600)
	}); err != nil {
		t.Fatal(err)
	}

	fallbackContent, fallbackMode := readFileContentAndMode(t, fallbackDst)
	cloneContent, cloneMode := readFileContentAndMode(t, cloneDst)
	if fallbackContent != cloneContent || fallbackMode != cloneMode {
		t.Fatalf("fallback output mismatch: content/mode %q %v vs %q %v", fallbackContent, fallbackMode, cloneContent, cloneMode)
	}
}

func assertCopiedFile(t *testing.T, path, wantContent string, wantMode fs.FileMode) {
	t.Helper()
	content, mode := readFileContentAndMode(t, path)
	if content != wantContent {
		t.Fatalf("content mismatch: want %q got %q", wantContent, content)
	}
	if mode != wantMode {
		t.Fatalf("mode mismatch: want %v got %v", wantMode, mode)
	}
}

func readFileContentAndMode(t *testing.T, path string) (string, fs.FileMode) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content), info.Mode().Perm()
}
