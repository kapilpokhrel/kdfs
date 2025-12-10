package kdfs

import (
	"context"
	"log/slog"
	"slices"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

type kdfsEntryDir struct {
	kdfsDir
}

func NewEntryDir(ctx context.Context, filename string, parent *fs.Inode, kdfsServer *KDFSServer) (*kdfsEntryDir, *fs.Inode) {
	var dir *kdfsEntryDir
	ch := parent.GetChild(filename)
	if ch == nil {
		dir = &kdfsEntryDir{kdfsDir{kdfsServer: kdfsServer}}
		ch = parent.NewPersistentInode(ctx, dir, fs.StableAttr{Mode: fuse.S_IFDIR})
		parent.AddChild(filename, ch, true)
		slog.Debug("Added a entry directory", "name", filename, "path", parent.Path(nil))
	} else {
		dir = ch.Operations().(*kdfsEntryDir)
	}

	dir.path = dir.Path(nil)
	return dir, ch
}

func (dir *kdfsEntryDir) getEntry() (gokeepasslib.Entry, error) {
	return dir.kdfsServer.DB.GetEntry(nil, cleanEntryPath(dir.path))
}

func (dir *kdfsEntryDir) setEntry(entry gokeepasslib.Entry) error {
	return dir.kdfsServer.DB.SetEntry(nil, cleanEntryPath(dir.path), entry)
}

func (dir *kdfsEntryDir) appendContent(name string, entry *gokeepasslib.Entry) syscall.Errno {
	keepassKey, ok := FsToKp[name]
	if !ok {
		return syscall.EINVAL
	}

	for i := range entry.Values {
		if keepassKey == entry.Values[i].Key {
			return syscall.EEXIST
		}
	}

	var protection bool
	if name == "password" {
		protection = true
	}
	entry.Values = append(
		entry.Values,
		gokeepasslib.ValueData{
			Key:   keepassKey,
			Value: gokeepasslib.V{Content: "", Protected: wrappers.NewBoolWrapper(protection)},
		},
	)
	return 0
}

var (
	_ = (fs.NodeGetattrer)((*kdfsEntryDir)(nil))
	_ = (fs.NodeCreater)((*kdfsEntryDir)(nil))
	_ = (fs.NodeUnlinker)((*kdfsEntryDir)(nil))
)

func (dir *kdfsEntryDir) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	perm := getPermission(ctx, dir.DefaultMode())
	if !hasWriteAccess(perm) {
		return nil, nil, 0, syscall.EPERM
	}

	logger := slog.Default().With("EntryDir", dir.path)

	var err error
	entry, err := dir.getEntry()
	if err != nil {
		logger.Error("Error findng a entry directory", "error", err)
		return nil, nil, 0, syscall.EIO
	}
	defer func() {
		if err = dir.setEntry(entry); err != nil {
			logger.Error("Error saving a entry", "error", err)
		}
	}()

	err = dir.appendContent(name, &entry)
	if err.(syscall.Errno) != 0 {
		return nil, nil, 0, err.(syscall.Errno)
	}
	fnode, child := NewFieldFile(ctx, name, &dir.Inode, dir.kdfsServer)

	currTime := wrappers.Now()
	entry.Times.LastModificationTime = &currTime

	logger.Debug("Create", "name", name)

	var attrOut fuse.AttrOut
	fnode.BaseAttr(&attrOut, entry.Times)
	out.AttrValid = attrOut.AttrValid
	out.EntryValid = attrOut.AttrValid
	out.EntryValidNsec = attrOut.AttrValidNsec
	out.AttrValidNsec = attrOut.AttrValidNsec
	out.Attr = attrOut.Attr

	return child, fnode, fnode.BaseFlag(), 0
}

func (dir *kdfsEntryDir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("EntryDir", dir.path)

	entry, err := dir.kdfsServer.DB.GetEntry(nil, cleanEntryPath(dir.path))
	if err != nil {
		logger.Error("Error findng a entry directory", "error", err)
		return syscall.EIO
	}

	dir.BaseAttr(out, entry.Times)
	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (dir *kdfsEntryDir) Unlink(ctx context.Context, name string) syscall.Errno {
	perm := getPermission(ctx, dir.DefaultMode())
	if !hasWriteAccess(perm) {
		return syscall.EPERM
	}

	logger := slog.Default().With("EntryDir", dir.path)

	logger.Debug("Unlink", "name", name)
	keepassKey, ok := FsToKp[name]
	if !ok {
		return syscall.EINVAL
	}

	entry, err := dir.getEntry()
	if err != nil {
		logger.Error("Error findng a entry directory", "error", err)
		return syscall.EIO
	}
	defer func() {
		if err = dir.setEntry(entry); err != nil {
			logger.Error("Error saving a entry", "error", err)
		}
	}()

	deleteIndex := 1
	for i, value := range entry.Values {
		if value.Key == keepassKey {
			deleteIndex = i
		}
	}
	if deleteIndex >= 0 {
		entry.Values = slices.Delete(entry.Values, deleteIndex, deleteIndex+1)
	} else {
		return syscall.ENOENT
	}
	return 0
}
