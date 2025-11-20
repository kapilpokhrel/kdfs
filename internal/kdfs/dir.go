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

	group *gokeepasslib.Group
	mu    sync.RWMutex
}

type kdfsEntryDir struct {
	fs.Inode

	entry *gokeepasslib.Entry
	mu    sync.RWMutex
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
	logger := slog.Default().With("GroupDir", dir.Path(nil))
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
		dir.mu.Lock()
		dir.group.Entries = append(dir.group.Entries, entry)
		dir.mu.Unlock()
		/* append copies a local entry to a group list
		   when we do &entry like (ddEntry(ctx, &dir.Inode, &entry)),
		   we pass the pointer to local copy, not the one stored in list */
		addEntry(ctx, &dir.Inode, &dir.group.Entries[len(dir.group.Entries)-1])
		traverseModifiedTime(&dir.Inode, &currTime)
		return dir.Children()[name], 0
	}
	logger.Debug("MkGroup", "group:", name)
	group := gokeepasslib.NewGroup()
	group.Name = name
	currTime := wrappers.Now()
	group.Times.CreationTime = &currTime
	group.Times.LastModificationTime = &currTime
	group.Times.LastAccessTime = &currTime
	dir.mu.Lock()
	dir.group.Groups = append(dir.group.Groups, group)
	dir.mu.Unlock()
	addGroup(ctx, &dir.Inode, &dir.group.Groups[len(dir.group.Groups)-1])
	traverseModifiedTime(&dir.Inode, &currTime)
	return dir.Children()[name], 0
}

func (dir *kdfsGroupDir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("GroupDir", dir.Path(nil))

	out.Mode = uint32(0o755)
	out.Nlink = 1

	if dir.group != nil { // group is nil in root
		dir.mu.RLock()
		out.Mtime = uint64(dir.group.Times.LastModificationTime.Time.Unix())
		out.Atime = uint64(dir.group.Times.LastAccessTime.Time.Unix())
		out.Ctime = uint64(dir.group.Times.CreationTime.Time.Unix())
		dir.mu.RUnlock()
	}

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (dir *kdfsEntryDir) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	logger := slog.Default().With("EntryDir", dir.Path(nil))
	keepassKey, ok := fsToKP[name]
	if !ok {
		return nil, nil, 0, syscall.EINVAL
	}

	for i := range dir.entry.Values {
		if keepassKey == dir.entry.Values[i].Key {
			return nil, nil, 0, syscall.EEXIST
		}
	}

	protection := false
	if name == "password" {
		protection = true
	}
	dir.entry.Values = append(
		dir.entry.Values,
		gokeepasslib.ValueData{
			Key:   keepassKey,
			Value: gokeepasslib.V{Content: "", Protected: wrappers.NewBoolWrapper(protection)},
		},
	)
	dataFile := memfile.New(0)
	fnode := &kdfsFieldFile{entry: dir.entry, data: dataFile, mu: &dir.Inode.Operations().(*kdfsEntryDir).mu}
	logger.Debug("Create", "name", name)

	out.Mode = uint32(0o0640)
	out.Nlink = 1

	dir.mu.RLock()
	out.Mtime = uint64(dir.entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(dir.entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(dir.entry.Times.CreationTime.Time.Unix())
	out.Size = uint64(0)
	dir.mu.RUnlock()

	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs

	child := dir.NewPersistentInode(ctx, fnode, fs.StableAttr{})
	dir.AddChild(
		name,
		child,
		true,
	)

	currTime := wrappers.Now()
	traverseModifiedTime(&dir.Inode, &currTime)
	rflags := uint32(fuse.FOPEN_KEEP_CACHE | fuse.O_ANYWRITE | fuse.FOPEN_DIRECT_IO | fuse.FOPEN_NOFLUSH)
	return child, fnode, rflags, 0
}

func (dir *kdfsEntryDir) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("EntryDir", dir.Path(nil))

	out.Mode = uint32(0o555)
	out.Nlink = 1

	dir.mu.RLock()
	out.Mtime = uint64(dir.entry.Times.LastModificationTime.Time.Unix())
	out.Atime = uint64(dir.entry.Times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(dir.entry.Times.CreationTime.Time.Unix())
	dir.mu.RUnlock()

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}
