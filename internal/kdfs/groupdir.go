package kdfs

import (
	"context"
	"log/slog"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

type kdfsGroupDir struct {
	kdfsDir
}

func NewGroupDir(ctx context.Context, filename string, parent *fs.Inode, kdfsServer *KDFSServer) (*kdfsGroupDir, *fs.Inode) {
	var dir *kdfsGroupDir
	ch := parent.GetChild(filename)
	if ch == nil {
		dir = &kdfsGroupDir{kdfsDir{kdfsServer: kdfsServer}}
		ch = parent.NewPersistentInode(ctx, dir, fs.StableAttr{Mode: fuse.S_IFDIR})
		parent.AddChild(filename, ch, true)
		slog.Debug("Added a group directory", "name", filename, "path", parent.Path(nil))
	} else {
		dir = ch.Operations().(*kdfsGroupDir)
	}

	dir.path = dir.Path(nil)
	return dir, ch
}

var (
	_ = (fs.NodeGetattrer)((*kdfsGroupDir)(nil))
	_ = (fs.NodeMkdirer)((*kdfsGroupDir)(nil))
)

func (dir *kdfsGroupDir) getGroup() (gokeepasslib.Group, error) {
	return dir.kdfsServer.DB.GetGroup(nil, dir.path)
}

func (dir *kdfsGroupDir) setEntry(group gokeepasslib.Group) error {
	return dir.kdfsServer.DB.SetGroup(nil, dir.path, group)
}

func (dir *kdfsGroupDir) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	logger := slog.Default().With("GroupDir", dir.path)

	group, err := dir.getGroup()
	if err != nil {
		logger.Error("Error findng a group", "error", err)
		return nil, syscall.EIO
	}
	defer func() {
		if err = dir.setEntry(group); err != nil {
			logger.Error("Error saving a group", "error", err)
		}
	}()

	if strings.HasSuffix(name, ".entry") {
		entryName, _ := strings.CutSuffix(name, ".entry")
		logger.Debug("MkEntry", "entry:", entryName)

		entry := gokeepasslib.NewEntry()
		entry.Values = append(
			entry.Values,
			gokeepasslib.ValueData{
				Key:   "Title",
				Value: gokeepasslib.V{Content: entryName, Protected: wrappers.NewBoolWrapper(false)},
			},
		)
		entry.Times = gokeepasslib.NewTimeData()

		group.Entries = append(group.Entries, entry)
		group.Times.LastModificationTime = entry.Times.CreationTime

		addEntry(ctx, &dir.Inode, entry, dir.kdfsServer)
		return dir.Children()[name], 0
	}
	logger.Debug("MkGroup", "group:", name)

	childGroup := gokeepasslib.NewGroup()
	childGroup.Name = name
	childGroup.Times = gokeepasslib.NewTimeData()

	group.Groups = append(group.Groups, childGroup)
	group.Times.LastModificationTime = childGroup.Times.CreationTime

	addGroup(ctx, &dir.Inode, childGroup, dir.kdfsServer)
	return dir.Children()[name], 0
}

func (dir *kdfsGroupDir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	if dir.path == "" {
		return 0
	}
	logger := slog.Default().With("GroupDir", dir.path)

	group, err := dir.kdfsServer.DB.GetGroup(nil, dir.path)
	if err != nil {
		logger.Error("Error findng a group", "error", err)
		return syscall.EIO
	}

	dir.BaseAttr(out, group.Times)
	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}
