package kdfs

import (
	"os"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tobischo/gokeepasslib/v3"
)

type kdfsDir struct {
	fs.Inode

	path       string
	kdfsServer *KDFSServer
}

func (dir *kdfsDir) DefaultMode() uint32 {
	return uint32(0o755)
}

func (dir *kdfsDir) BaseAttr(out *fuse.AttrOut, times gokeepasslib.TimeData) {
	out.AttrValid = fuse.FATTR_ATIME | fuse.FATTR_MTIME | fuse.FATTR_CTIME | fuse.FATTR_UID | fuse.FATTR_GID
	out.Mode = dir.DefaultMode()
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())
	out.Mtime = uint64(times.LastModificationTime.Time.Unix())
	out.Atime = uint64(times.LastAccessTime.Time.Unix())
	out.Ctime = uint64(times.CreationTime.Time.Unix())
}
