package kdfs

import (
	"sync"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/tobischo/gokeepasslib/v3"
)

type kdfsDir struct {
	fs.Inode

	group *gokeepasslib.Group
	mu    sync.RWMutex
}
