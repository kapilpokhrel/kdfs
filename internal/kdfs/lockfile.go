package kdfs

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/kdbx"
	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

type kdfsLockStateFile struct {
	kdfsFile

	times gokeepasslib.TimeData
}

var (
	_ = (fs.NodeGetattrer)((*kdfsLockStateFile)(nil))
	_ = (fs.NodeReader)((*kdfsLockStateFile)(nil))
)

func NewLockStateFile(ctx context.Context, filename string, parent *fs.Inode, kdfsServer *KDFSServer) (*kdfsLockStateFile, *fs.Inode) {
	var file *kdfsLockStateFile
	ch := parent.GetChild(filename)
	if ch == nil {
		file = &kdfsLockStateFile{kdfsFile: kdfsFile{kdfsServer: kdfsServer}, times: gokeepasslib.NewTimeData()}
		ch = parent.NewPersistentInode(ctx, file, fs.StableAttr{})
		parent.AddChild(filename, ch, true)
		slog.Debug("Added a entry directory", "name", filename, "path", parent.Path(nil))
	} else {
		file = ch.Operations().(*kdfsLockStateFile)
	}

	path := file.Path(nil)
	file.path = path
	return file, ch
}

func (file *kdfsLockStateFile) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("lock_state_file", file.path)

	file.BaseAttr(out, file.times)
	out.Mode = uint32(0o0440)
	out.Size = uint64(2)

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (file *kdfsLockStateFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	logger := slog.Default().With("lock_state_file", file.path)

	if flags&uint32(os.O_APPEND|os.O_WRONLY|os.O_RDWR|os.O_TRUNC) != 0 {
		return nil, 0, syscall.EPERM
	}

	rflags := file.BaseFlag()
	rflags &= ^fuse.O_ANYWRITE
	logger.Debug("Open", "inflags", flags, "outflags", rflags)

	return fs.FileHandle(file), rflags, 0
}

func (file *kdfsLockStateFile) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	logger := slog.Default().With("lock_state_file", file.path)
	logger.Debug("Read", "offset", off, "len", len(dest))

	currTime := wrappers.Now()
	file.times.LastAccessTime = &currTime

	if off > 0 {
		return fuse.ReadResultData([]byte{}), 0
	}

	if file.kdfsServer.DB.GetState() {
		return fuse.ReadResultData([]byte("t")), 0
	} else {
		return fuse.ReadResultData([]byte("f")), 0
	}
}

type kdfsLockActionFile struct {
	kdfsFile

	times gokeepasslib.TimeData
}

var (
	_ = (fs.NodeGetattrer)((*kdfsFieldFile)(nil))
	_ = (fs.NodeWriter)((*kdfsLockActionFile)(nil))
)

func NewLockActionFile(ctx context.Context, filename string, parent *fs.Inode, kdfsServer *KDFSServer) (*kdfsLockActionFile, *fs.Inode) {
	var file *kdfsLockActionFile
	ch := parent.GetChild(filename)
	if ch == nil {
		file = &kdfsLockActionFile{kdfsFile: kdfsFile{kdfsServer: kdfsServer}, times: gokeepasslib.NewTimeData()}
		ch = parent.NewPersistentInode(ctx, file, fs.StableAttr{})
		parent.AddChild(filename, ch, true)
		slog.Debug("Added a lock action file")
	} else {
		file = ch.Operations().(*kdfsLockActionFile)
	}

	path := file.Path(nil)
	file.path = path
	return file, ch
}

func (file *kdfsLockActionFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	logger := slog.Default().With("lock_state_file", file.path)

	if flags&uint32(os.O_RDONLY|os.O_RDWR|os.O_TRUNC) != 0 {
		return nil, 0, syscall.EPERM
	}
	rflags := file.BaseFlag()
	rflags &= ^fuse.O_ANYWRITE
	logger.Debug("Open", "inflags", flags, "outflags", rflags)

	return fs.FileHandle(file), rflags, 0
}

func (file *kdfsLockActionFile) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	logger := slog.Default().With("lock_action_file", file.path)

	file.BaseAttr(out, file.times)
	out.AttrValid = out.AttrValid ^ fuse.FATTR_SIZE
	out.Mode = uint32(0o0200)

	logger.Debug("GetAttr", slog.Group("OutAttr", "Mode", out.Mode, "Size", out.Size))
	return 0
}

func (file *kdfsLockActionFile) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	logger := slog.Default().With("lock_action_file", file.path)
	logger.Debug("Write", "offset", off, "len", len(data))

	if off != 0 {
		return 0, syscall.EPERM
	}
	if len(data) == 0 || len(data) > 72 {
		return 0, syscall.EPERM
	}

	currTime := wrappers.Now()
	if strings.ToLower(strings.TrimSpace(string(data))) == "lock" {
		err := file.kdfsServer.DB.Lock()
		if err != nil && !errors.Is(err, kdbx.ErrAlreadyExist) {
			return 0, syscall.EIO
		}
		file.times.LastModificationTime = &currTime
		return 4, 0
	}

	if verifyHashPassword(data, file.kdfsServer.passwordHash) {
		err := file.kdfsServer.DB.Unlock()
		if err != nil && !errors.Is(err, kdbx.ErrAlreadyExist) {
			return 0, syscall.EIO
		}
		file.times.LastModificationTime = &currTime
		return uint32(len(data)), 0
	} else {
		return 0, syscall.EACCES
	}
}
