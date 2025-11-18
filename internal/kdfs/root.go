// Package kdfs implements a types and methods for KDBS filesystem around gokeepasslib and go-fuse
package kdfs

import (
	"context"
	"log/slog"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/memfile"
	"github.com/tobischo/gokeepasslib/v3"
)

type kdfsRoot struct {
	kdfsDir

	kdfsServer *KDFSServer
	root       *gokeepasslib.RootData
}

/*
addEntry and addGroup needs to get the address of group and entry using index
(not ranging over Entries and Groups) to keep reference to the same variable in the
original DB
*/

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

	for _, valueData := range e.Values {
		fname, ok := kpToFS[valueData.Key]
		if !ok {
			continue
		}
		content := valueData.Value.Content
		if len(content) == 0 {
			continue
		}

		dataFile := memfile.New(len(content))
		dataFile.WriteAt([]byte(content), 0)
		fnode := &kdfsFile{entry: e, data: dataFile}
		ch.AddChild(
			fname,
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
	for i := range g.Groups {
		addGroup(ctx, ch, &g.Groups[i])
	}
	for i := range g.Entries {
		addEntry(ctx, ch, &g.Entries[i])
	}
}

var _ = (fs.NodeOnAdder)((*kdfsRoot)(nil))

func (kdfs *kdfsRoot) OnAdd(ctx context.Context) {
	r := &kdfs.Inode

	for i := range kdfs.root.Groups {
		addGroup(ctx, r, &kdfs.root.Groups[i])
	}
}
