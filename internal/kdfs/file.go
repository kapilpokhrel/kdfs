package kdfs

import (
	"context"
	"log/slog"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/memfile"
	"github.com/tobischo/gokeepasslib/v3"
)

type kdfsFile struct {
	fs.Inode

	data  *memfile.MemFile
	entry *gokeepasslib.Entry
	mu    sync.RWMutex
}

var (
	_ = (fs.NodeOpener)((*kdfsFile)(nil))
	_ = (fs.NodeGetattrer)((*kdfsFile)(nil))
	_ = (fs.NodeReader)((*kdfsFile)(nil))
	_ = (fs.NodeSetattrer)((*kdfsFile)(nil))
	_ = (fs.NodeCreater)((*kdfsFile)(nil))
)

func (file *kdfsFile) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	flogger := slog.Default().With("file", file.Path(nil))

	out.Mode = uint32(0o0640)
	out.Nlink = 1
	out.Mtime = uint64(file.entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(file.entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(file.entry.Times.CreationTime.Time.Unix())
	out.Size = uint64(file.data.Len())

	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs
	flogger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (file *kdfsFile) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	flogger := slog.Default().With("file", file.Path(nil))

	out.Mode = in.Mode
	out.Nlink = 1

	// Update lastModificationTime
	out.Mtime = uint64(file.entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(file.entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(file.entry.Times.CreationTime.Time.Unix())
	out.Size = in.Size

	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs
	flogger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (file *kdfsFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	flogger := slog.Default().With("file", file.Path(nil))

	// Update lastaccessTime, OR SHOUD WE DO IT ON CLOSE? RELEASE?
	rflags := uint32(fuse.FOPEN_CACHE_DIR | fuse.O_ANYWRITE)
	flogger.Debug("Open", "inflags", flags, "outflags", rflags)
	return fs.FileHandle(file), rflags, 0
}

func (file *kdfsFile) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	flogger := slog.Default().With("file", file.Path(nil))

	flogger.Debug("Read", "offset", off, "len", len(dest))

	n, err := file.data.ReadAt(dest, off)
	if err != 0 {
		return nil, err
	}
	return fuse.ReadResultData(dest[:n]), 0
}

func (file *kdfsFile) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	flogger := slog.Default().With("file", file.Path(nil))

	flogger.Debug("Create", "name", name, "flags", flags, "mode", mode)
	return nil, nil, 0, 0
}

func (file *kdfsFile) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	flogger := slog.Default().With("file", file.Path(nil))

	// Update lastModificationTime
	flogger.Debug("Write", "offset", off, "len", len(data))
	n, err := file.data.WriteAt(data, off)
	return uint32(n), err
}
