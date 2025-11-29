package kdfs

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/kapilpokhrel/kdfs/internal/kdbx"
)

type KDFSServer struct {
	DB           *kdbx.Database
	Server       *fuse.Server
	passwordHash []byte
	openTime     time.Time
	Cfg          KDFSConfig
}

type KDFSConfig struct {
	MountPoint       string
	KDBXValutPath    string
	saveOnExit       bool
	autoLockDuration time.Duration
}

func AddFlags(flagset *flag.FlagSet, cfg *KDFSConfig) {
	flagset.BoolVar(&cfg.saveOnExit, "saveonexit", true, "Save on Exit")
	flagset.DurationVar(&cfg.autoLockDuration, "autolockduration", 5*time.Minute, "auto lock duration")
}

func getCopyPath(kdbxfile string) string {
	dirPath, filename := filepath.Split(kdbxfile)
	copyFilename := fmt.Sprintf("%s.%d.tmp", filename, os.Getpid())
	copyFilepath := filepath.Join(dirPath, copyFilename)

	return copyFilepath
}

func copyKdbxFile(kdbxfile string) (string, error) {
	srcfile, err := os.Open(kdbxfile)
	if err != nil {
		return "", err
	}
	defer srcfile.Close()

	copyFilepath := getCopyPath(kdbxfile)
	copyfile, err := os.Create(copyFilepath)
	if err != nil {
		return "", err
	}
	defer copyfile.Close()

	_, err = io.Copy(copyfile, srcfile)
	if err != nil {
		return "", err
	}
	return copyFilepath, nil
}

func NewKDFSServer(cfg KDFSConfig, password []byte) (*KDFSServer, error) {
	if len(cfg.MountPoint) == 0 {
		return nil, errors.New("empty mountpoint")
	}
	if len(cfg.KDBXValutPath) == 0 {
		return nil, errors.New("empty kdbx valut path")
	}

	copyFilepath, err := copyKdbxFile(cfg.KDBXValutPath)
	if err != nil {
		return nil, err
	}
	db, err := kdbx.Open(copyFilepath, password)
	if err != nil {
		return nil, err
	}

	err = db.Unlock()
	if err != nil {
		return nil, errors.Join(errors.New("incorrect credential"), err)
	}

	hash, _ := hashPassword(password)
	kdfsServer := &KDFSServer{DB: db, passwordHash: hash, Cfg: cfg}

	kdbsRoot := &kdfsRoot{root: db.Root()}
	kdbsRoot.kdfsServer = kdfsServer

	server, err := fs.Mount(cfg.MountPoint, kdbsRoot, &fs.Options{})
	if err != nil {
		return nil, errors.Join(errors.New("mount failed"), err)
	}

	kdfsServer.Server = server
	kdfsServer.openTime = time.Now()
	if cfg.autoLockDuration != 0 {
		go db.StartAutoLockTicker(cfg.autoLockDuration, 10*time.Second)
	}
	return kdfsServer, nil
}

func (s *KDFSServer) syncToOriginal() {
	// This is the side effect so it doesn't return error but rather just logs it
	copyPath := getCopyPath(s.Cfg.KDBXValutPath)
	logger := slog.Default().With("Auto Copy Aborted (do the manual copy)", copyPath)
	originalFileStat, err := os.Stat(s.Cfg.KDBXValutPath)
	if err != nil {
		logger.Error(fmt.Sprintf("Error getting original file stat %v", err))
		return
	}

	if originalFileStat.ModTime().After(s.openTime) {
		logger.Error("Original file modified after opening")
		return
	}

	if err := os.Rename(copyPath, s.Cfg.KDBXValutPath); err != nil {
		logger.Error(fmt.Sprintf("Error renaming the working file to original file %v", err))
		return
	}

	slog.Info("Original kdbxfile updated", "filepath", s.Cfg.KDBXValutPath)
}

func (s *KDFSServer) Umount() error {
	return s.Server.Unmount()
}

func (s *KDFSServer) Wait() {
	s.Server.Wait()
	s.DB.CloseCn <- true
	if s.Cfg.saveOnExit {
		s.syncToOriginal()
	}
}
