package kdfs

import (
	"strings"
	"syscall"
)

func ReverseMap(m map[string]string) map[string]string {
	r := make(map[string]string, len(m))
	for k, v := range m {
		r[v] = k
	}
	return r
}

var kpToFS = map[string]string{
	"UserName": "username",
	"Password": "password",
	"URL":      "url",
	"Notes":    "notes",
}

var fsToKP = ReverseMap(kpToFS)

func cleanEntryPath(entryPath string) string {
	splits := strings.Split(entryPath, ".entry")
	return splits[0]
}

func readAt(src []byte, off int64, dest []byte) (int, syscall.Errno) {
	if off < 0 {
		return 0, syscall.EINVAL
	}
	if off >= int64(len(src)) {
		return 0, 0 // EOF
	}
	n := copy(dest, src[off:])
	return n, 0
}

func writeAt(dest []byte, off int64, data []byte) ([]byte, int, syscall.Errno) {
	if off < 0 {
		return dest, 0, syscall.EINVAL
	}

	need := int(off) + len(data)
	if need > len(dest) {
		grow := make([]byte, need)
		copy(grow, dest)
		dest = grow
	}

	n := copy(dest[off:], data)
	return dest, n, 0
}

func setSize(dest []byte, n int64) ([]byte, syscall.Errno) {
	if n < 0 {
		return dest, syscall.EINVAL
	}

	if int64(len(dest)) >= n {
		return dest[:n], 0
	}

	needed := n - int64(len(dest))
	dest = append(dest, make([]byte, needed)...)
	return dest, 0
}
