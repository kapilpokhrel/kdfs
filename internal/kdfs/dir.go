package kdfs

import (
	"context"
	"log/slog"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tobischo/gokeepasslib/v3"
)

type kdfsGroupDir struct {
	fs.Inode

	group *gokeepasslib.Group
	mu    sync.RWMutex
}

type kdfsEntryDir struct {
	fs.Inode

	entry *gokeepasslib.Entry
	mu    sync.RWMutex
}

var (
	_ = (fs.NodeGetattrer)((*kdfsGroupDir)(nil))
	_ = (fs.NodeMkdirer)((*kdfsGroupDir)(nil))
)

var (
	_ = (fs.NodeGetattrer)((*kdfsEntryDir)(nil))
	_ = (fs.NodeCreater)((*kdfsEntryDir)(nil))
)

func (dir *kdfsGroupDir) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	return nil, syscall.ENOSYS
}

func (dir *kdfsGroupDir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("GroupDir", dir.Path(nil))

	out.Mode = uint32(0o755)
	out.Nlink = 1

	if dir.group != nil { // group is nul in root
		dir.mu.RLock()
		out.Mtime = uint64(dir.group.Times.LastModificationTime.Time.Unix())
		out.Atime = uint64(dir.group.Times.LastAccessTime.Time.Unix())
		out.Ctime = uint64(dir.group.Times.CreationTime.Time.Unix())
		dir.mu.RUnlock()
	}

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (*kdfsEntryDir) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	return nil, nil, 0, syscall.ENOSYS
}

func (dir *kdfsEntryDir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("EntryDir", dir.Path(nil))

	out.Mode = uint32(0o555)
	out.Nlink = 1

	dir.mu.RLock()
	out.Mtime = uint64(dir.entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(dir.entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(dir.entry.Times.CreationTime.Time.Unix())
	dir.mu.RUnlock()

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}
