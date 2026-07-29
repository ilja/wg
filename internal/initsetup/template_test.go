package initsetup

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTemplatePath(t *testing.T) {
	t.Run("uses absolute XDG config home", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "config")
		got, err := ResolveTemplatePath([]string{"HOME=/ignored", "XDG_CONFIG_HOME=" + root})
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(root, "wg", "setup.sh")
		if got != want {
			t.Fatalf("want %q, got %q", want, got)
		}
	})

	t.Run("falls back to injected home", func(t *testing.T) {
		home := t.TempDir()
		got, err := ResolveTemplatePath([]string{"UNRELATED=value", "XDG_CONFIG_HOME=", "HOME=" + home})
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(home, ".config", "wg", "setup.sh")
		if got != want {
			t.Fatalf("want %q, got %q", want, got)
		}
	})

	t.Run("ignores malformed environment entries", func(t *testing.T) {
		home := t.TempDir()
		got, err := ResolveTemplatePath([]string{"XDG_CONFIG_HOME", "HOME=" + home})
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(home, ".config", "wg", "setup.sh")
		if got != want {
			t.Fatalf("want %q, got %q", want, got)
		}
	})

	t.Run("rejects relative XDG config home", func(t *testing.T) {
		_, err := ResolveTemplatePath([]string{"XDG_CONFIG_HOME=relative", "HOME=/home/test"})
		if err == nil || !strings.Contains(err.Error(), "XDG_CONFIG_HOME") {
			t.Fatalf("expected XDG_CONFIG_HOME diagnostic, got %v", err)
		}
	})

	t.Run("requires injected home", func(t *testing.T) {
		_, err := ResolveTemplatePath([]string{"PATH=/bin"})
		if err == nil || !strings.Contains(err.Error(), "HOME") {
			t.Fatalf("expected HOME diagnostic, got %v", err)
		}
	})
}
