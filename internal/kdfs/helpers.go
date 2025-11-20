package kdfs

import (
	"log/slog"

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

func traverseModifiedTime(node *fs.Inode, time *wrappers.TimeWrapper) {
	if node == nil {
		return
	}
	switch n := node.Operations().(type) {
	case *kdfsRoot:
		logger := slog.Default()
		err := n.kdfsServer.DB.Save(n.kdfsServer.kdbxfilepath)
		if err != nil {
			logger.Error("Save failed", "error", err)
		}
		return
	case *kdfsFieldFile:
		n.mu.Lock()
		n.entry.Times.LastModificationTime = time
		n.mu.Unlock()
	case *kdfsGroupDir:
		n.mu.Lock()
		n.group.Times.LastModificationTime = time
		n.mu.Unlock()
	case *kdfsEntryDir:
		n.mu.Lock()
		n.entry.Times.LastModificationTime = time
		n.mu.Unlock()
	}
	_, parent := node.Parent()
	traverseModifiedTime(parent, time)
}

func traverseAccessTime(node *fs.Inode, time *wrappers.TimeWrapper) {
	if node == nil {
		return
	}

	switch n := node.Operations().(type) {
	case *kdfsRoot:
		logger := slog.Default()
		err := n.kdfsServer.DB.Save(n.kdfsServer.kdbxfilepath)
		if err != nil {
			logger.Error("Save failed", "error", err)
		}
		return
	case *kdfsFieldFile:
		n.mu.Lock()
		n.entry.Times.LastAccessTime = time
		n.mu.Unlock()
	case *kdfsGroupDir:
		n.mu.Lock()
		n.group.Times.LastAccessTime = time
		n.mu.Unlock()
	}
	_, parent := node.Parent()
	traverseAccessTime(parent, time)
}
