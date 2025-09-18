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
	"github.com/kapilpokhrel/kdfs/pkg/multih"
	"github.com/lmittmann/tint"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

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
	flag.BoolVar(&daemon, "daemon", false, "Run as a background daemon")
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

	var pass []byte

	// Demoanized execution
	if os.Getenv("DAEMON") != "1" {
		fmt.Fprintln(os.Stderr, "Enter Password: ")

		var err error
		pass, err = term.ReadPassword(int(os.Stdin.Fd()))
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
	} else {
		pass, _ = io.ReadAll(os.Stdin)
	}

	server, err := kdfs.NewKDFSServer(flag.Arg(1), pass, flag.Arg(0))
	if err != nil {
		slog.Error("Failed to create a kdfs server", "error", err)
		os.Exit(1)
	}
	pass = nil
	server.DB.Lock()

	fmt.Printf("Mounted KDFS at %s\n", flag.Arg(0))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		server.Umount()
	}()
	server.Wait()
}
