// Package kdfs implements a types and methods for KDBS filesystem around gokeepasslib and go-fuse
package kdfs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tobischo/gokeepasslib/v3"
)

type kdfsRoot struct {
	kdfsGroupDir

	root *gokeepasslib.RootData
}

func addEntry(ctx context.Context, parent *fs.Inode, e gokeepasslib.Entry, server *KDFSServer) {
	title := e.GetTitle()
	if len(title) == 0 {
		slog.Debug("Skipping entry because it has no title", "url", e.GetContent("URL"), "user", e.GetContent("Username"))
		return
	}
	filename := fmt.Sprintf("%s.entry", title)

	_, ch := NewEntryDir(ctx, filename, parent, server)

	for _, valueData := range e.Values {
		fname, ok := KpToFs[valueData.Key]
		if !ok {
			continue
		}
		NewFieldFile(ctx, fname, ch, server)
	}
}

func addGroup(ctx context.Context, parent *fs.Inode, g gokeepasslib.Group, server *KDFSServer) {
	_, ch := NewGroupDir(ctx, g.Name, parent, server)
	for i := range g.Groups {
		addGroup(ctx, ch, g.Groups[i], server)
	}
	for i := range g.Entries {
		addEntry(ctx, ch, g.Entries[i], server)
	}
}

var (
	_ = (fs.NodeOnAdder)((*kdfsRoot)(nil))
	_ = (fs.NodeGetattrer)((*kdfsRoot)(nil))
)

func (root *kdfsRoot) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("Root", "/")

	out.AttrValid = fuse.FATTR_ATIME | fuse.FATTR_MTIME | fuse.FATTR_CTIME | fuse.FATTR_UID | fuse.FATTR_GID
	out.Mode = root.DefaultMode()
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())
	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode))
	return 0
}

func (root *kdfsRoot) OnAdd(ctx context.Context) {
	r := &root.Inode

	for i := range root.root.Groups {
		addGroup(ctx, r, root.root.Groups[i], root.kdfsServer)
	}
	NewLockStateFile(ctx, "lockstate", r, root.kdfsServer)
	NewLockActionFile(ctx, "lockaction", r, root.kdfsServer)
}
