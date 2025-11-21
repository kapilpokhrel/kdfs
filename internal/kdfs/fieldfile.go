package kdfs

import (
	"context"
	"log/slog"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

/* We can create a kdfsEntryFile which keeps the entry as read only json file which all the fields inside it. For a normal natural entry like file */

type kdfsFieldFile struct {
	kdfsFile
}

var (
	_ = (fs.NodeOpener)((*kdfsFieldFile)(nil))
	_ = (fs.NodeGetattrer)((*kdfsFieldFile)(nil))
	_ = (fs.NodeReader)((*kdfsFieldFile)(nil))
	_ = (fs.NodeSetattrer)((*kdfsFieldFile)(nil))
)

/* All files in the entry gets the same creation, modified, access time from a common entry */

func NewFieldFile(ctx context.Context, filename string, parent *fs.Inode, kdfsServer *KDFSServer) (*kdfsFieldFile, *fs.Inode) {
	var file *kdfsFieldFile
	ch := parent.GetChild(filename)
	if ch == nil {
		file = &kdfsFieldFile{kdfsFile{kdfsServer: kdfsServer}}
		ch = parent.NewPersistentInode(ctx, file, fs.StableAttr{})
		parent.AddChild(filename, ch, true)
		slog.Debug("Added a entry directory", "name", filename, "path", parent.Path(nil))
	} else {
		file = ch.Operations().(*kdfsFieldFile)
	}

	path := file.Path(nil)
	file.path = path
	return file, ch
}

func (file *kdfsFieldFile) getEntry() (gokeepasslib.Entry, error) {
	return file.kdfsServer.DB.GetEntry(nil, cleanEntryPath(file.path))
}

func (file *kdfsFieldFile) setEntry(entry gokeepasslib.Entry) error {
	return file.kdfsServer.DB.SetEntry(nil, cleanEntryPath(file.path), entry)
}

func (file *kdfsFieldFile) getContent(entry gokeepasslib.Entry) string {
	fname := filepath.Base(file.path)
	keepassKey := fsToKP[fname]
	return entry.GetContent(keepassKey)
}

func (file *kdfsFieldFile) setContent(entry *gokeepasslib.Entry, newContent []byte) {
	fname := filepath.Base(file.path)
	keepassKey := fsToKP[fname]
	for i := range entry.Values {
		if keepassKey == entry.Values[i].Key {
			entry.Values[i].Value.Content = string(newContent)
			break
		}
	}
}

func (file *kdfsFieldFile) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("field_file", file.path)

	entry, err := file.getEntry()
	if err != nil {
		logger.Error("Erorr in getting a entry")
		return syscall.EIO
	}

	content := file.getContent(entry)

	file.BaseAttr(out, entry.Times)
	out.Size = uint64(len(content))

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (file *kdfsFieldFile) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("field_file", file.path)

	entry, err := file.getEntry()
	if err != nil {
		logger.Error("Erorr in getting a entry")
		return syscall.EIO
	}

	changed := false
	defer func() {
		if !changed {
			return
		}
		if err = file.setEntry(entry); err != nil {
			logger.Error("Error in saving a entry")
		}
	}()

	if in.Valid&fuse.FATTR_MTIME != 0 {
		modifiedTime := wrappers.Now()
		entry.Times.LastModificationTime = &modifiedTime

		changed = true
	}
	if in.Valid&fuse.FATTR_SIZE != 0 {
		out.Size = in.Size

		oldContent := file.getContent(entry)
		newContent, err := setSize([]byte(entry.GetContent(oldContent)), int64(in.Size))
		if err != 0 {
			return err
		}
		file.setContent(&entry, newContent)
		changed = true
	}
	file.BaseAttr(out, entry.Times)
	logger.Debug("SetAttr", slog.Group("InAttr", "Mode", in.Mode, "Size", out.Size), slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))

	return 0
}

func (file *kdfsFieldFile) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	logger := slog.Default().With("field_file", file.path)
	logger.Debug("Read", "offset", off, "len", len(dest))

	var err error

	entry, err := file.getEntry()
	if err != nil {
		logger.Error("Erorr in getting a entry")
		return nil, syscall.EIO
	}
	content := file.getContent(entry)
	n, err := readAt([]byte(content), off, dest)
	if err.(syscall.Errno) != 0 {
		return nil, err.(syscall.Errno)
	}

	if n != 0 {
		defer func() {
			if err = file.setEntry(entry); err != nil {
				logger.Error("Error in saving a entry")
			}
		}()

		accessTime := wrappers.Now()
		entry.Times.LastAccessTime = &accessTime
	}

	return fuse.ReadResultData(dest[:n]), 0
}

func (file *kdfsFieldFile) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	logger := slog.Default().With("field_file", file.path)
	logger.Debug("Write", "offset", off, "len", len(data))

	var err error

	entry, err := file.getEntry()
	if err != nil {
		logger.Error("Erorr in getting a entry")
		return 0, syscall.EIO
	}
	defer func() {
		if err = file.setEntry(entry); err != nil {
			logger.Error("Error in saving a entry")
		}
	}()

	oldContent := file.getContent(entry)
	newContent, n, err := writeAt([]byte(oldContent), off, data)
	if err.(syscall.Errno) != 0 {
		return 0, err.(syscall.Errno)
	}
	file.setContent(&entry, newContent)

	modifiedTime := wrappers.Now()
	entry.Times.LastModificationTime = &modifiedTime

	return uint32(n), 0
}
