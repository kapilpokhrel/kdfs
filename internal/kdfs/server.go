package kdfs

import (
	"errors"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/kdbx"
)

type KDFSServer struct {
	kdbxfilepath string
	DB           *kdbx.Database
	Server       *fuse.Server
	Mount        string
}

func NewKDFSServer(kdbxfile string, password []byte, mountpoint string) (*KDFSServer, error) {
	db, err := kdbx.Open(kdbxfile, password)
	if err != nil {
		return nil, err
	}

	err = db.Unlock()
	if err != nil {
		return nil, errors.Join(errors.New("incorrect credential"), err)
	}

	kdbsRoot := &kdfsRoot{root: db.Root()}
	server, err := fs.Mount(mountpoint, kdbsRoot, &fs.Options{})
	if err != nil {
		return nil, errors.Join(errors.New("mount failed"), err)
	}

	return &KDFSServer{kdbxfilepath: kdbxfile, DB: db, Server: server, Mount: mountpoint}, nil
}

func (s *KDFSServer) Umount() {
	s.Server.Unmount()
}

func (s *KDFSServer) Wait() {
	s.Server.Wait()
}
