package kdfs

import (
	"errors"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/kdbx"
)

type KDFSServer struct {
	DB           *kdbx.Database
	Server       *fuse.Server
	Mount        string
	passwordHash []byte
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

	hash, _ := hashPassword(password)
	kdfsServer := &KDFSServer{DB: db, Mount: mountpoint, passwordHash: hash}

	kdbsRoot := &kdfsRoot{root: db.Root()}
	kdbsRoot.kdfsServer = kdfsServer

	server, err := fs.Mount(mountpoint, kdbsRoot, &fs.Options{})
	if err != nil {
		return nil, errors.Join(errors.New("mount failed"), err)
	}

	kdfsServer.Server = server
	return kdfsServer, nil
}

func (s *KDFSServer) Umount() {
	s.Server.Unmount()
}

func (s *KDFSServer) Wait() {
	s.Server.Wait()
}
