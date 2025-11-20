// Package kdfs implements a types and methods for KDBS filesystem around gokeepasslib and go-fuse
package kdfs

import (
	"context"
	"fmt"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/memfile"
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
	ch := parent.GetChild(filename)
	if ch == nil {
		ch = parent.NewPersistentInode(ctx, &kdfsEntryDir{kdfsServer: server}, fs.StableAttr{Mode: fuse.S_IFDIR})
		parent.AddChild(filename, ch, true)
		slog.Debug("Added a entry directory", "title", title, "path", parent.Path(nil))
	}

	for _, valueData := range e.Values {
		fname, ok := kpToFS[valueData.Key]
		if !ok {
			continue
		}
		content := valueData.Value.Content

		dataFile := memfile.New(len(content))
		dataFile.WriteAt([]byte(content), 0)
		fnode := &kdfsFieldFile{data: dataFile, kdfsServer: server}
		ch.AddChild(
			fname,
			ch.NewPersistentInode(ctx, fnode, fs.StableAttr{}),
			true,
		)
	}
}

func addGroup(ctx context.Context, parent *fs.Inode, g gokeepasslib.Group, server *KDFSServer) {
	ch := parent.GetChild(g.Name)
	if ch == nil {
		ch = parent.NewPersistentInode(ctx, &kdfsGroupDir{kdfsServer: server}, fs.StableAttr{Mode: syscall.S_IFDIR | syscall.S_IREAD | syscall.S_IWRITE})
		parent.AddChild(g.Name, ch, true)
		slog.Debug("Added a group directory", "name", g.Name, "path", parent.Path(nil))
	}
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
}
