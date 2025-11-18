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

func (dir *kdfsFieldFile) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	flogger := slog.Default().With("file", dir.Path(nil))

	out.Mode = in.Mode
	out.Nlink = 1

	modifiedTime := wrappers.Now()
	out.Mtime = uint64(modifiedTime.Time.Unix())
	dir.mu.RLock()
	out.Atime = uint64(dir.entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(dir.entry.Times.CreationTime.Time.Unix())
	dir.mu.RUnlock()
	out.Size = in.Size

	dir.mu.Lock()
	dir.data.SetSize(int64(in.Size))
	dir.mu.Unlock()

	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs
	flogger.Debug("SetAttr", slog.Group("InAttr", "Mode", in.Mode, "Size", out.Size), slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))

	traverseModefiedTime(&dir.Inode, &modifiedTime)
	return 0
}

func (dir *kdfsFieldFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	flogger := slog.Default().With("file", dir.Path(nil))

	// Update lastaccessTime, OR SHOUD WE DO IT ON CLOSE? RELEASE?
	rflags := uint32(fuse.FOPEN_CACHE_DIR | fuse.O_ANYWRITE)
	flogger.Debug("Open", "inflags", flags, "outflags", rflags)
	return fs.FileHandle(dir), rflags, 0
}

func (dir *kdfsFieldFile) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	flogger := slog.Default().With("file", dir.Path(nil))

	flogger.Debug("Read", "offset", off, "len", len(dest))

	dir.mu.RLock()
	n, err := dir.data.ReadAt(dest, off)
	dir.mu.RUnlock()
	if err != 0 {
		return nil, err
	}

	accessTime := wrappers.Now()
	traverseaccessTime(&dir.Inode, &accessTime)

	return fuse.ReadResultData(dest[:n]), 0
}

func (dir *kdfsFieldFile) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	flogger := slog.Default().With("file", dir.Path(nil))

	flogger.Debug("Write", "offset", off, "len", len(data))
	n, err := dir.data.WriteAt(data, off)

	fname := filepath.Base(dir.Path(nil))
	keepassKey := fsToKP[fname]

	for i := range dir.entry.Values {
		if keepassKey == dir.entry.Values[i].Key {
			dir.mu.Lock()
			dir.entry.Values[i].Value.Content = string(dir.data.Bytes())
			dir.mu.Unlock()
			modifiedTime := wrappers.Now()
			traverseModefiedTime(&dir.Inode, &modifiedTime)
			break
		}
	}
	return uint32(n), err
}
