package kdfs

import (
	"context"
	"os"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
	"golang.org/x/crypto/bcrypt"
)

func ReverseMap(m map[string]string) map[string]string {
	r := make(map[string]string, len(m))
	for k, v := range m {
		r[v] = k
	}
	return r
}

var KpToFs = map[string]string{
	"UserName": "username",
	"Password": "password",
	"URL":      "url",
	"Notes":    "notes",
}

var FsToKp = ReverseMap(KpToFs)

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

func hashPassword(password []byte) ([]byte, error) {
	bytes, err := bcrypt.GenerateFromPassword(password, 10)
	return bytes, err
}

func verifyHashPassword(password, hash []byte) bool {
	err := bcrypt.CompareHashAndPassword(hash, password)
	return err == nil
}

func getPermission(ctx context.Context, mode uint32) uint32 {
	caller := ctx.(*fuse.Context).Caller

	uid := caller.Uid
	gid := caller.Gid

	var perm uint32 = mode & 0o007
	switch {
	case uid == uint32(os.Getgid()):
		perm = (mode & 0o700) >> 6
	case gid == uint32(os.Getgid()):
		perm = (mode & 0o070) >> 3
	default:
		supplementaryGroups, err := syscall.Getgroups()
		if err == nil {
			for _, group := range supplementaryGroups {
				if uint32(group) == gid {
					// The user is a member of the group, apply group permissions
					perm = (mode & 0o070) >> 3
					break
				}
			}
		}
	}

	return perm
}

func hasReadAccess(perm uint32) bool {
	return perm&4 != 0
}

func hasWriteAccess(perm uint32) bool {
	return perm&2 != 0
}

func checkOpenPerm(perm uint32, flags uint32) bool {
	// We don't check execute
	if !hasReadAccess(perm) && flags&uint32(os.O_RDONLY|os.O_RDWR) != 0 {
		return false
	}
	if !hasWriteAccess(perm) && flags&uint32(os.O_APPEND|os.O_WRONLY|os.O_RDWR|os.O_TRUNC) != 0 {
		return false
	}
	return true
}

func verifyOpenPermission(ctx context.Context, flags uint32, mode uint32) bool {
	perm := getPermission(ctx, mode)
	return checkOpenPerm(perm, flags)
}
