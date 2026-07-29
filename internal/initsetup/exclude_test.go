package initsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureExcludeAddsPattern(t *testing.T) {
	for _, existing := range []string{"", "# local excludes\n"} {
		t.Run(existing, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "exclude")
			if existing != "" {
				if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := EnsureExclude(path, "/.config/"); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if want := existing + "/.config/\n"; string(got) != want {
				t.Fatalf("want %q, got %q", want, got)
			}
		})
	}
}

func TestEnsureExcludePreservesExistingContent(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		want     string
	}{
		{name: "final newline", existing: "# local\n\n*.tmp\n", want: "# local\n\n*.tmp\n/.config/\n"},
		{name: "no final newline", existing: "# local\n*.tmp", want: "# local\n*.tmp\n/.config/\n"},
		{name: "near matches", existing: ".config/\n/.config\n /.config/\n", want: ".config/\n/.config\n /.config/\n/.config/\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "exclude")
			if err := os.WriteFile(path, []byte(test.existing), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := EnsureExclude(path, "/.config/"); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("want %q, got %q", test.want, got)
			}
		})
	}
}

func TestEnsureExcludeIsIdempotentForExactEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exclude")
	existing := "# local\n/.config/\n\n*.tmp\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureExclude(path, "/.config/"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureExclude(path, "/.config/"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != existing {
		t.Fatalf("existing content changed: want %q, got %q", existing, got)
	}
}

func TestEnsureExcludeReportsInvalidParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "exclude")
	err := EnsureExclude(path, "/.config/")
	if err == nil {
		t.Fatal("expected missing parent error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("expected error to name %q, got %v", path, err)
	}
}

func TestEnsureExcludeReportsAppendFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exclude")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	err := EnsureExclude(path, "/.config/")
	if err == nil {
		t.Fatal("expected append failure")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("expected error to name %q, got %v", path, err)
	}
}
