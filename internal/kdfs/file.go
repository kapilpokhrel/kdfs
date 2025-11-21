package kdfs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

/* We can create a kdfsEntryFile which keeps the entry as read only json file which all the fields inside it. For a normal natural entry like file */

type kdfsFieldFile struct {
	fs.Inode

	mu         sync.RWMutex
	kdfsServer *KDFSServer
}

var (
	_ = (fs.NodeOpener)((*kdfsFieldFile)(nil))
	_ = (fs.NodeGetattrer)((*kdfsFieldFile)(nil))
	_ = (fs.NodeReader)((*kdfsFieldFile)(nil))
	_ = (fs.NodeSetattrer)((*kdfsFieldFile)(nil))
)

/* All files in the entry gets the same creation, modified, access time from a common entry */

func (file *kdfsFieldFile) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	path := file.Path(nil)
	logger := slog.Default().With("file", path)

	out.Mode = uint32(0o0640)
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())
	out.Nlink = 1

	entry, err := file.kdfsServer.DB.GetEntry(nil, cleanEntryPath(path))
	if err != nil {
		logger.Error("Erorr in getting a entry")
		return syscall.EIO
	}
	fname := filepath.Base(file.Path(nil))
	keepassKey := fsToKP[fname]
	content := entry.GetContent(keepassKey)

	out.Mtime = uint64(entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(entry.Times.CreationTime.Time.Unix())
	out.Size = uint64(len(content))
	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (file *kdfsFieldFile) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	path := file.Path(nil)
	logger := slog.Default().With("file", path)

	entry, err := file.kdfsServer.DB.GetEntry(nil, cleanEntryPath(path))
	if err != nil {
		logger.Error("Erorr in getting a entry")
		return syscall.EIO
	}

	changed := false
	defer func() {
		if !changed {
			return
		}
		if err = file.kdfsServer.DB.SetEntry(nil, cleanEntryPath(path), entry); err != nil {
			logger.Error("Error in saving a entry")
		}
	}()

	out.Mode = uint32(0o0640)
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())

	out.Nlink = 1
	if in.Valid&fuse.FATTR_MTIME != 0 {
		modifiedTime := wrappers.Now()
		out.Mtime = uint64(modifiedTime.Time.Unix())
		entry.Times.LastModificationTime = &modifiedTime

		changed = true
	}
	out.Atime = uint64(entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(entry.Times.CreationTime.Time.Unix())

	if in.Valid&fuse.FATTR_SIZE != 0 {
		out.Size = in.Size
		fname := filepath.Base(file.Path(nil))
		keepassKey := fsToKP[fname]
		newContent, err := setSize([]byte(entry.GetContent(keepassKey)), int64(in.Size))
		if err != 0 {
			return err
		}
		for i := range entry.Values {
			if keepassKey == entry.Values[i].Key {
				file.mu.RLock()
				entry.Values[i].Value.Content = string(newContent)
				file.mu.RUnlock()
				break
			}
		}

		changed = true
	}
	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs
	logger.Debug("SetAttr", slog.Group("InAttr", "Mode", in.Mode, "Size", out.Size), slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))

	return 0
}

func (file *kdfsFieldFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	logger := slog.Default().With("file", file.Path(nil))

	rflags := uint32(fuse.FOPEN_KEEP_CACHE | fuse.O_ANYWRITE | fuse.FOPEN_DIRECT_IO | fuse.FOPEN_NOFLUSH)
	logger.Debug("Open", "inflags", flags, "outflags", rflags)
	return fs.FileHandle(file), rflags, 0
}

func (file *kdfsFieldFile) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	path := file.Path(nil)
	logger := slog.Default().With("file", path)
	logger.Debug("Read", "offset", off, "len", len(dest))

	var err error

	entry, err := file.kdfsServer.DB.GetEntry(nil, cleanEntryPath(path))
	if err != nil {
		logger.Error("Erorr in getting a entry")
		return nil, syscall.EIO
	}
	fname := filepath.Base(file.Path(nil))
	keepassKey := fsToKP[fname]
	content := entry.GetContent(keepassKey)
	n, err := readAt([]byte(content), off, dest)
	if err.(syscall.Errno) != 0 {
		return nil, err.(syscall.Errno)
	}

	if n != 0 {
		defer func() {
			if err = file.kdfsServer.DB.SetEntry(nil, cleanEntryPath(path), entry); err != nil {
				logger.Error("Error in saving a entry")
			}
		}()

		accessTime := wrappers.Now()
		entry.Times.LastAccessTime = &accessTime
	}

	return fuse.ReadResultData(dest[:n]), 0
}

func (file *kdfsFieldFile) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	path := file.Path(nil)
	logger := slog.Default().With("file", path)
	logger.Debug("Write", "offset", off, "len", len(data))

	var err error

	entry, err := file.kdfsServer.DB.GetEntry(nil, cleanEntryPath(path))
	if err != nil {
		logger.Error("Erorr in getting a entry")
		return 0, syscall.EIO
	}
	defer func() {
		if err = file.kdfsServer.DB.SetEntry(nil, cleanEntryPath(path), entry); err != nil {
			logger.Error("Error in saving a entry")
		}
	}()

	fname := filepath.Base(file.Path(nil))
	keepassKey := fsToKP[fname]
	oldContent := entry.GetContent(keepassKey)
	newContent, n, err := writeAt([]byte(oldContent), off, data)
	if err.(syscall.Errno) != 0 {
		return 0, err.(syscall.Errno)
	}

	for i := range entry.Values {
		if keepassKey == entry.Values[i].Key {
			file.mu.RLock()
			entry.Values[i].Value.Content = string(newContent)
			file.mu.RUnlock()
			break
		}
	}
	modifiedTime := wrappers.Now()
	entry.Times.LastModificationTime = &modifiedTime

	return uint32(n), 0
}
