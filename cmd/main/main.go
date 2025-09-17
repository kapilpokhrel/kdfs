// Package main implements a cli tool to open kdbx file and mount it as a filesystem
package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/kapilpokhrel/kdfs/internal/kdfs"
	"github.com/kapilpokhrel/kdfs/pkg/multih"
	"github.com/lmittmann/tint"
	"github.com/tobischo/gokeepasslib/v3"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

func OpenKDBX(kdbxFile string, password []byte) (db *gokeepasslib.Database, err error) {
	file, err := os.Open(kdbxFile)
	if err != nil {
		return
	}
	db = gokeepasslib.NewDatabase()
	db.Credentials = gokeepasslib.NewPasswordCredentials(string(password))
	err = gokeepasslib.NewDecoder(file).Decode(db)
	if err != nil {
		return
	}
	return db, nil
}

func startKDFS(kdbxpath string, mountpoint string, password []byte) {
	db, err := OpenKDBX(kdbxpath, password)
	if err != nil {
		slog.Error("Can't open kdbx file", "error", err)
		os.Exit(1)
	}

	err = db.UnlockProtectedEntries()
	if err != nil {
		slog.Error("Incorrect credential", "error", err)
		os.Exit(1)
	}

	kdbsRoot := kdfs.NewKDFSRoot(db.Content.Root)
	server, err := fs.Mount(mountpoint, kdbsRoot, &fs.Options{})
	if err != nil {
		slog.Error("Couldn't mount the fs on the given mount point", "mountpoint", flag.Arg(0), "error", err)
		os.Exit(1)
	}
	db.LockProtectedEntries()
	password = nil
	server.Wait()
}

func setupLogger() {
	stdHandler := tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelWarn})

	homeDir, _ := os.UserHomeDir()
	rotFileWriter := &lumberjack.Logger{
		Filename:   filepath.Join(homeDir, ".local/share/kdfs/logs/kdfs.log"),
		MaxSize:    50, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	}
	rotFileHandler := slog.NewTextHandler(rotFileWriter, &slog.HandlerOptions{Level: slog.LevelInfo})

	multiH := multih.NewMultiHandler(stdHandler, rotFileHandler)
	logger := slog.New(multiH)
	slog.SetDefault(logger)
}

func main() {
	setupLogger()

	// Flags
	var daemon bool
	flag.BoolVar(&daemon, "daemon", true, "Run as a background daemon")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s [ options ] <mountpoint> <vault (kdbx file)>\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Options:")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(2)
	}

	// Demoanized execution
	if os.Getenv("DAEMON") != "1" {
		fmt.Fprint(os.Stderr, "Enter Password: ")
		pass, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			slog.Error("Couldn't read password from user", "error", err)
			os.Exit(1)
		}

		if daemon {
			if r, w, err := os.Pipe(); err == nil {
				w.Write(pass)
				w.Close()

				cmd := exec.Command(os.Args[0], os.Args[1:]...)
				cmd.Env = append(os.Environ(), "DAEMON=1")
				cmd.Stdin = r
				cmd.Stdout = nil
				cmd.Stderr = nil
				cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
				if err := cmd.Start(); err == nil {
					cmd.Process.Release()
					os.Exit(0)
				}
				slog.Warn("Couldn't start FS daemon", "error", err)
			} else {
				slog.Warn("Couldn't open pipe for FS daemon", "error", err)
			}
		}
		startKDFS(flag.Arg(1), flag.Arg(0), pass)
		os.Exit(0)
	}
	pass, _ := io.ReadAll(os.Stdin)
	startKDFS(flag.Arg(1), flag.Arg(0), pass)
}
