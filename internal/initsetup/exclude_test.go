package initsetup

import (
	"os"
	"path/filepath"
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
