package initsetup

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ResolveTemplatePath(environ []string) (string, error) {
	xdgConfigHome := environmentValue(environ, "XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		if !filepath.IsAbs(xdgConfigHome) {
			return "", fmt.Errorf("XDG_CONFIG_HOME must be an absolute path, got %q", xdgConfigHome)
		}
		return filepath.Join(xdgConfigHome, "wg", "setup.sh"), nil
	}

	home := environmentValue(environ, "HOME")
	if home == "" {
		return "", fmt.Errorf("HOME is not set; cannot resolve reusable setup template")
	}
	return filepath.Join(home, ".config", "wg", "setup.sh"), nil
}

func environmentValue(environ []string, key string) string {
	prefix := key + "="
	for _, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}
