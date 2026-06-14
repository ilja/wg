//go:build darwin

package copier

import (
	"errors"

	"golang.org/x/sys/unix"
)

func cloneFile(src string, dst string) error {
	if err := unix.Clonefile(src, dst, 0); err != nil {
		if isUnsupportedCloneErr(err) {
			return ErrCloneUnsupported
		}
		return err
	}
	return nil
}

func isUnsupportedCloneErr(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EXDEV)
}
