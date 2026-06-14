package worktree

import (
	"fmt"
	"path/filepath"
	"strings"
)

func SanitizeWorktreeName(branch string) (string, error) {
	var builder strings.Builder
	lastUnsafe := false

	for _, r := range branch {
		if isSafeNameRune(r) {
			builder.WriteRune(r)
			lastUnsafe = false
			continue
		}
		if !lastUnsafe {
			builder.WriteByte('-')
			lastUnsafe = true
		}
	}

	name := strings.Trim(builder.String(), "-")
	if name == "" {
		return "", fmt.Errorf("worktree name for branch %q is empty after sanitizing", branch)
	}
	return name, nil
}

func SiblingWorktreePath(primaryWorktreePath string, branch string) (name string, path string, err error) {
	name, err = SanitizeWorktreeName(branch)
	if err != nil {
		return "", "", err
	}

	primary := filepath.Clean(primaryWorktreePath)
	path = filepath.Join(filepath.Dir(primary), filepath.Base(primary)+"."+name)
	return name, path, nil
}

func isSafeNameRune(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		r == '.' || r == '_' || r == '-'
}
