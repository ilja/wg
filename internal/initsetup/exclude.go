package initsetup

import (
	"bytes"
	"fmt"
	"os"
)

func EnsureExclude(path, pattern string) error {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read Git exclude file %q: %w", path, err)
	}
	for _, line := range bytes.Split(content, []byte("\n")) {
		if string(line) == pattern {
			return nil
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open Git exclude file %q: %w", path, err)
	}
	defer file.Close()

	if len(content) > 0 && content[len(content)-1] != '\n' {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("append separator to Git exclude file %q: %w", path, err)
		}
	}
	if _, err := file.WriteString(pattern + "\n"); err != nil {
		return fmt.Errorf("append pattern to Git exclude file %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Git exclude file %q: %w", path, err)
	}
	return nil
}
