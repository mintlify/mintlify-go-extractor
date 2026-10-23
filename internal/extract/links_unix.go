//go:build unix

package extract

import (
	"os"
	"syscall"
)

// linkCount reports how many directory entries refer to the file.
func linkCount(info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink)
	}
	return 1
}
