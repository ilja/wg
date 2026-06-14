package ports_test

import (
	"testing"

	"wg/internal/ports"
)

func TestDerivePort(t *testing.T) {
	path := "/repo/demo.feature-alpha"
	first := ports.DerivePort(path)
	second := ports.DerivePort(path)
	if first != second {
		t.Fatalf("expected stable port for same path, got %d and %d", first, second)
	}
	if first < 10000 || first > 19999 {
		t.Fatalf("expected port in 10000-19999, got %d", first)
	}
	other := ports.DerivePort("/repo/demo.feature-beta")
	if first == other {
		t.Fatalf("expected path-sensitive ports for fixed test paths, both were %d", first)
	}
}
