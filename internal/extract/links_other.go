//go:build !unix

package extract

import "os"

// linkCount reports how many directory entries refer to the file. Windows
// exposes this only through the Win32 file handle API; checkouts there are
// git working trees, which never contain hard links, so one is assumed.
func linkCount(os.FileInfo) uint64 {
	return 1
}
