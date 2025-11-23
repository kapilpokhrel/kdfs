// Package kdfs implements a types and methods for KDBS filesystem around gokeepasslib and go-fuse
package kdfs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hanwen/go-fuse/v2/fs"
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

var _ = (fs.NodeOnAdder)((*kdfsRoot)(nil))

func (kdfs *kdfsRoot) OnAdd(ctx context.Context) {
	r := &kdfs.Inode

	for i := range kdfs.root.Groups {
		addGroup(ctx, r, kdfs.root.Groups[i], kdfs.kdfsServer)
	}
	NewLockStateFile(ctx, "lockstate", r, kdfs.kdfsServer)
	NewLockActionFile(ctx, "lockaction", r, kdfs.kdfsServer)
}
