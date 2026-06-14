package ports

import (
	"hash/fnv"
	"path/filepath"
)

func DerivePort(path string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(filepath.Clean(path)))
	return 10000 + int(h.Sum32()%10000)
}
