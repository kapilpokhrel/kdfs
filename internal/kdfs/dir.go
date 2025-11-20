package kdfs

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/memfile"
	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

type kdfsGroupDir struct {
	fs.Inode

	kdfsServer *KDFSServer
	mu         sync.RWMutex
}

type kdfsEntryDir struct {
	fs.Inode

	kdfsServer *KDFSServer
	mu         sync.RWMutex
}

var (
	_ = (fs.NodeGetattrer)((*kdfsGroupDir)(nil))
	_ = (fs.NodeMkdirer)((*kdfsGroupDir)(nil))
)

var (
	_ = (fs.NodeGetattrer)((*kdfsEntryDir)(nil))
	_ = (fs.NodeCreater)((*kdfsEntryDir)(nil))
)

func (dir *kdfsGroupDir) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	path := dir.Path(nil)
	logger := slog.Default().With("GroupDir", path)

	group, err := dir.kdfsServer.DB.GetGroup(nil, path)
	if err != nil {
		logger.Error("Error findng a group", "error", err)
		return nil, syscall.EIO
	}
	defer func() {
		if err = dir.kdfsServer.DB.SetGroup(nil, path, group); err != nil {
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
		currTime := wrappers.Now()
		entry.Times.CreationTime = &currTime
		entry.Times.LastModificationTime = &currTime
		entry.Times.LastAccessTime = &currTime

		group.Entries = append(group.Entries, entry)
		group.Times.LastModificationTime = &currTime

		addEntry(ctx, &dir.Inode, entry, dir.kdfsServer)
		return dir.Children()[name], 0
	}
	logger.Debug("MkGroup", "group:", name)

	childGroup := gokeepasslib.NewGroup()
	childGroup.Name = name
	currTime := wrappers.Now()
	childGroup.Times.CreationTime = &currTime
	childGroup.Times.LastModificationTime = &currTime
	childGroup.Times.LastAccessTime = &currTime

	group.Groups = append(group.Groups, childGroup)
	group.Times.LastModificationTime = &currTime

	addGroup(ctx, &dir.Inode, childGroup, dir.kdfsServer)
	return dir.Children()[name], 0
}

func (dir *kdfsGroupDir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	path := dir.Path(nil)
	if path == "" {
		return 0
	}
	logger := slog.Default().With("GroupDir", path)

	group, err := dir.kdfsServer.DB.GetGroup(nil, path)
	if err != nil {
		logger.Error("Error findng a group", "error", err)
		return syscall.EIO
	}

	out.Mode = uint32(0o755)
	out.Nlink = 1
	out.Mtime = uint64(group.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(group.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(group.Times.CreationTime.Time.Unix())

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (dir *kdfsEntryDir) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	path := dir.Path(nil)
	logger := slog.Default().With("EntryDir", path)

	entry, err := dir.kdfsServer.DB.GetEntry(nil, cleanEntryPath(path))
	if err != nil {
		logger.Error("Error findng a entry directory", "error", err)
		return nil, nil, 0, syscall.EIO
	}
	defer func() {
		if err = dir.kdfsServer.DB.SetEntry(nil, cleanEntryPath(path), entry); err != nil {
			logger.Error("Error saving a entry", "error", err)
		}
	}()

	keepassKey, ok := fsToKP[name]
	if !ok {
		return nil, nil, 0, syscall.EINVAL
	}

	for i := range entry.Values {
		if keepassKey == entry.Values[i].Key {
			return nil, nil, 0, syscall.EEXIST
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
	currTime := wrappers.Now()
	entry.Times.LastModificationTime = &currTime

	fnode := &kdfsFieldFile{data: memfile.New(0), kdfsServer: dir.kdfsServer}

	logger.Debug("Create", "name", name)

	out.Mode = uint32(0o0640)
	out.Nlink = 1
	out.Mtime = uint64(entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(entry.Times.CreationTime.Time.Unix())

	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs

	child := dir.NewPersistentInode(ctx, fnode, fs.StableAttr{})
	dir.AddChild(
		name,
		child,
		true,
	)
	logger.Debug("Added a entry field", "name", name)

	rflags := uint32(fuse.FOPEN_KEEP_CACHE | fuse.O_ANYWRITE | fuse.FOPEN_DIRECT_IO | fuse.FOPEN_NOFLUSH)
	return child, fnode, rflags, 0
}

func (dir *kdfsEntryDir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	path := dir.Path(nil)
	logger := slog.Default().With("EntryDir", path)

	entry, err := dir.kdfsServer.DB.GetEntry(nil, cleanEntryPath(path))
	if err != nil {
		logger.Error("Error findng a entry directory", "error", err)
		return syscall.EIO
	}

	out.Mode = uint32(0o555)
	out.Nlink = 1
	out.Mtime = uint64(entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(entry.Times.CreationTime.Time.Unix())

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}
