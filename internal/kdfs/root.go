// Package kdfs implements a types and methods for KDBS filesystem around gokeepasslib and go-fuse
package kdfs

import (
	"context"
	"log/slog"
	"strings"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/memfile"
	"github.com/tobischo/gokeepasslib/v3"
)

type kdfsRoot struct {
	kdfsDir

	root *gokeepasslib.RootData
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

		dataFile := memfile.New(len(content))
		dataFile.WriteAt([]byte(content), 0)
		fnode := &kdfsFile{entry: e, data: dataFile}
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
