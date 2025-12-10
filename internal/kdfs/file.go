package kdfs

import (
	"context"
	"log/slog"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tobischo/gokeepasslib/v3"
)

type kdfsFile struct {
	fs.Inode

	path       string
	kdfsServer *KDFSServer
}

func (file *kdfsFile) DefaultMode() uint32 {
	return uint32(0o0640)
}

func (file *kdfsFile) BaseAttr(out *fuse.AttrOut, times gokeepasslib.TimeData) {
	out.AttrValid = fuse.FATTR_ATIME | fuse.FATTR_MTIME | fuse.FATTR_CTIME | fuse.FATTR_UID | fuse.FATTR_GID | fuse.FATTR_SIZE
	out.AttrValidNsec = 0
	out.Mode = file.DefaultMode()
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())
	out.Mtime = uint64(times.LastModificationTime.Time.Unix())
	out.Atime = uint64(times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(times.CreationTime.Time.Unix())
}

func (file *kdfsFile) BaseFlag() uint32 {
	flags := uint32(fuse.O_ANYWRITE | fuse.FOPEN_DIRECT_IO | fuse.FOPEN_NOFLUSH)
	flags &= ^uint32(fuse.FOPEN_KEEP_CACHE)
	return flags
}

func (file *kdfsFieldFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	logger := slog.Default().With("file", file.path)

	logger.Debug("Open", "inflags", flags, "outflags", file.BaseFlag())

	if !verifyOpenPermission(ctx, flags, file.DefaultMode()) {
		return nil, 0, syscall.EPERM
	}
	return fs.FileHandle(file), file.BaseFlag(), 0
}
