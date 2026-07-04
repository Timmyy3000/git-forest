//go:build !windows

package state

import "os"

func atomicReplace(src, dst string) error {
	return os.Rename(src, dst)
}
