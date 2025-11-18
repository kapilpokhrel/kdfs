package kdfs

import (
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

func ReverseMap(m map[string]string) map[string]string {
	r := make(map[string]string, len(m))
	for k, v := range m {
		r[v] = k
	}
	return r
}

var kpToFS = map[string]string{
	"UserName": "username",
	"Password": "password",
	"URL":      "url",
	"Notes":    "notes",
}

var fsToKP = ReverseMap(kpToFS)

func traverseModefiedTime(node *fs.Inode, time *wrappers.TimeWrapper) {
	if node == nil {
		return
	}
	switch n := node.Operations().(type) {
	case *kdfsRoot:
		n.kdfsServer.DB.Save(n.kdfsServer.kdbxfilepath)
		return
	case *kdfsFile:
		n.mu.Lock()
		n.entry.Times.LastModificationTime = time
		n.mu.Unlock()
	case *kdfsDir:
		n.mu.Lock()
		n.group.Times.LastModificationTime = time
		n.mu.Unlock()
	}
	_, parent := node.Parent()
	traverseModefiedTime(parent, time)
}

func traverseaccessTime(node *fs.Inode, time *wrappers.TimeWrapper) {
	if node == nil {
		return
	}

	switch n := node.Operations().(type) {
	case *kdfsRoot:
		n.kdfsServer.DB.Save(n.kdfsServer.kdbxfilepath)
		return
	case *kdfsFile:
		n.mu.Lock()
		n.entry.Times.LastAccessTime = time
		n.mu.Unlock()
	case *kdfsDir:
		n.mu.Lock()
		n.group.Times.LastAccessTime = time
		n.mu.Unlock()
	}
	_, parent := node.Parent()
	traverseaccessTime(parent, time)
}
