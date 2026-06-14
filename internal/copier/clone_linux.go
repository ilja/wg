//go:build linux

package copier

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func cloneFile(src string, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer destination.Close()

	if err := unix.IoctlFileClone(int(destination.Fd()), int(source.Fd())); err != nil {
		if isUnsupportedCloneErr(err) {
			return ErrCloneUnsupported
		}
		return err
	}
	return nil
}

func isUnsupportedCloneErr(err error) bool {
	return errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOTTY) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EXDEV)
}
