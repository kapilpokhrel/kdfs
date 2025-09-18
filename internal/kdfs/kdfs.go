// Package kdfs implements a types and methods for KDBS filesystem around gokeepasslib and go-fuse
package kdfs

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tobischo/gokeepasslib/v3"
)

type kdfsDir struct {
	fs.Inode

	group *gokeepasslib.Group
	mu    sync.RWMutex
}

type kdfsRoot struct {
	kdfsDir

	root *gokeepasslib.RootData
}

type kdfsFile struct {
	fs.Inode

	data  []byte
	entry *gokeepasslib.Entry
	mu    sync.RWMutex
}

var (
	_ = (fs.NodeOpener)((*kdfsFile)(nil))
	_ = (fs.NodeGetattrer)((*kdfsFile)(nil))
	_ = (fs.NodeReader)((*kdfsFile)(nil))
)

func (file *kdfsFile) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	flogger := slog.Default().With("file", file.Path(nil))

	out.Mode = uint32(0o7777)
	out.Nlink = 1
	out.Mtime = uint64(file.entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(file.entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(file.entry.Times.CreationTime.Time.Unix())
	out.Size = uint64(len(file.data))

	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs
	flogger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (file *kdfsFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	flogger := slog.Default().With("file", file.Path(nil))

	rflags := uint32(fuse.FOPEN_CACHE_DIR | fuse.O_ANYWRITE)
	flogger.Debug("Open", "inflags", flags, "outflags", rflags)
	return fs.FileHandle(file), rflags, 0
}

func (file *kdfsFile) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	flogger := slog.Default().With("file", file.Path(nil))

	flogger.Debug("Read", "offset", off, "len", len(dest))
	end := min(int(off)+len(dest), len(file.data))
	return fuse.ReadResultData(file.data[off:end]), 0
}

func addEntry(ctx context.Context, parent *fs.Inode, e *gokeepasslib.Entry) {
	title := e.GetTitle()
	if len(title) == 0 {
		slog.Debug("Skipping entry because it has no title", "url", e.GetContent("URL"), "user", e.GetContent("Username"))
		return
	}
	ch := parent.GetChild(title)
	if ch == nil {
		ch = parent.NewPersistentInode(ctx, &kdfsDir{group: parent.Operations().(*kdfsDir).group}, fs.StableAttr{Mode: fuse.S_IFDIR})
		parent.AddChild(title, ch, true)
	}

	files := []string{"UserName", "Password", "Notes", "URL"}
	for _, f := range files {
		content := e.GetContent(f)
		if len(content) == 0 {
			continue
		}
		fnode := &kdfsFile{entry: e, data: []byte(content)}
		ch.AddChild(
			strings.ToLower(f),
			ch.NewPersistentInode(ctx, fnode, fs.StableAttr{}),
			true,
		)

	}
}

func addGroup(ctx context.Context, parent *fs.Inode, g *gokeepasslib.Group) {
	ch := parent.GetChild(g.Name)
	if ch == nil {
		ch = parent.NewPersistentInode(ctx, &kdfsDir{group: g}, fs.StableAttr{Mode: fuse.S_IFDIR})
		parent.AddChild(g.Name, ch, true)
	}
	for _, group := range g.Groups {
		addGroup(ctx, ch, &group)
	}
	for _, entry := range g.Entries {
		addEntry(ctx, ch, &entry)
	}
}

var _ = (fs.NodeOnAdder)((*kdfsRoot)(nil))

func (kdfs *kdfsRoot) OnAdd(ctx context.Context) {
	r := &kdfs.Inode

	for _, group := range kdfs.root.Groups {
		addGroup(ctx, r, &group)
	}
}
