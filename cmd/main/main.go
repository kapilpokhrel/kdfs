// Package main implements a cli tool to open kdbx file and mount it as a filesystem
package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kapilpokhrel/kdfs/internal/kdfs"
	"github.com/kapilpokhrel/kdfs/pkg/slog/handlermux"
	"github.com/lmittmann/tint"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

func setupLogger(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	stdHandler := tint.NewHandler(os.Stdout, &tint.Options{Level: level})

	homeDir, _ := os.UserHomeDir()
	rotFileWriter := &lumberjack.Logger{
		Filename:   filepath.Join(homeDir, ".local/share/kdfs/logs/kdfs.log"),
		MaxSize:    50, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	}
	rotFileHandler := slog.NewTextHandler(rotFileWriter, &slog.HandlerOptions{Level: slog.LevelInfo})

	multiH := handlermux.New(stdHandler, rotFileHandler)
	logger := slog.New(multiH)
	slog.SetDefault(logger)
}

func main() {
	// Flags
	flagset := flag.CommandLine
	var daemon bool
	var debug bool
	flagset.BoolVar(&daemon, "daemon", false, "Run as a background daemon")
	flagset.BoolVar(&debug, "debug", false, "Enable Debug Mode")

	var serverCfg kdfs.KDFSConfig
	kdfs.AddFlags(flagset, &serverCfg)

	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(),
			"Usage: %s [ options ] <mountpoint> <vault (kdbx file)>\n\n", os.Args[0])
		fmt.Fprintln(flagset.Output(), "Options:")
		flag.PrintDefaults()
	}

	flagset.Parse(os.Args[1:])

	args := flagset.Args()
	if len(args) < 2 {
		flag.Usage()
		os.Exit(2)
	}

	setupLogger(debug)
	var pass []byte

	// Demoanized execution
	if os.Getenv("DAEMON") != "1" {
		var err error
		pass = []byte(os.Getenv("KDBX_DB_MASTER_KEY"))
		if len(pass) == 0 {
			fmt.Fprintln(os.Stderr, "Enter Password: ")
			pass, err = term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				slog.Error("Couldn't read password from user", "error", err)
				os.Exit(1)
			}
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
	} else {
		pass, _ = io.ReadAll(os.Stdin)
	}

	serverCfg.MountPoint = args[0]
	serverCfg.KDBXValutPath = args[1]
	server, err := kdfs.NewKDFSServer(serverCfg, pass)
	if err != nil {
		slog.Error("Failed to create a kdfs server", "error", err)
		os.Exit(1)
	}
	pass = nil

	fmt.Printf("Mounted KDFS at %s\n", flag.Arg(0))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		server.Umount()
	}()
	server.Wait()
}
