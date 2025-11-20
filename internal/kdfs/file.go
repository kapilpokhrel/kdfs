package kdfs

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/memfile"
	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

/* We can create a kdfsEntryFile which keeps the entry as read only json file which all the fields inside it. For a normal natural entry like file */

type kdfsFieldFile struct {
	fs.Inode

	data  *memfile.MemFile
	entry *gokeepasslib.Entry
	mu    *sync.RWMutex
}

var (
	_ = (fs.NodeOpener)((*kdfsFieldFile)(nil))
	_ = (fs.NodeGetattrer)((*kdfsFieldFile)(nil))
	_ = (fs.NodeReader)((*kdfsFieldFile)(nil))
	_ = (fs.NodeSetattrer)((*kdfsFieldFile)(nil))
)

/* All files in the entry gets the same creation, modified, access time from a common entry */

func (file *kdfsFieldFile) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	flogger := slog.Default().With("file", file.Path(nil))

	out.Mode = uint32(0o0640)
	out.Nlink = 1

	file.mu.RLock()
	out.Mtime = uint64(file.entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(file.entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(file.entry.Times.CreationTime.Time.Unix())
	out.Size = uint64(file.data.Len())
	file.mu.RUnlock()

	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs
	flogger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (file *kdfsFieldFile) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	flogger := slog.Default().With("file", file.Path(nil))

	out.Mode = in.Mode
	out.Nlink = 1

	modifiedTime := wrappers.Now()
	out.Mtime = uint64(modifiedTime.Time.Unix())
	file.mu.RLock()
	out.Atime = uint64(file.entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(file.entry.Times.CreationTime.Time.Unix())
	file.mu.RUnlock()
	out.Size = in.Size

	file.mu.Lock()
	file.data.SetSize(int64(in.Size))
	file.mu.Unlock()

	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs
	flogger.Debug("SetAttr", slog.Group("InAttr", "Mode", in.Mode, "Size", out.Size), slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))

	traverseModifiedTime(&file.Inode, &modifiedTime)
	return 0
}

func (file *kdfsFieldFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	flogger := slog.Default().With("file", file.Path(nil))

	// Update lastaccessTime, OR SHOUD WE DO IT ON CLOSE? RELEASE?
	rflags := uint32(fuse.FOPEN_KEEP_CACHE | fuse.O_ANYWRITE | fuse.FOPEN_DIRECT_IO | fuse.FOPEN_NOFLUSH)
	flogger.Debug("Open", "inflags", flags, "outflags", rflags)
	return fs.FileHandle(file), rflags, 0
}

func (file *kdfsFieldFile) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	flogger := slog.Default().With("file", file.Path(nil))

	flogger.Debug("Read", "offset", off, "len", len(dest))

	file.mu.RLock()
	n, err := file.data.ReadAt(dest, off)
	file.mu.RUnlock()
	if err != 0 {
		return nil, err
	}

	if n != 0 {
		accessTime := wrappers.Now()
		traverseAccessTime(&file.Inode, &accessTime)
	}

	return fuse.ReadResultData(dest[:n]), 0
}

func (file *kdfsFieldFile) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	flogger := slog.Default().With("file", file.Path(nil))

	flogger.Debug("Write", "offset", off, "len", len(data))
	n, err := file.data.WriteAt(data, off)

	fname := filepath.Base(file.Path(nil))
	keepassKey := fsToKP[fname]

	for i := range file.entry.Values {
		if keepassKey == file.entry.Values[i].Key {
			file.mu.Lock()
			file.entry.Values[i].Value.Content = string(file.data.Bytes())
			file.mu.Unlock()
			modifiedTime := wrappers.Now()
			traverseModifiedTime(&file.Inode, &modifiedTime)
			break
		}
	}
	return uint32(n), err
}
