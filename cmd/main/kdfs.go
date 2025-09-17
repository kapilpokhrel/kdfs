// Package main implements a cli tool to open kdbx file and mount it as a filesystem
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kapilpokhrel/kdfs/pkg/multih"
	"github.com/lmittmann/tint"
	"github.com/tobischo/gokeepasslib/v3"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

func OpenKDBX(kdbxFile string, password string) (db *gokeepasslib.Database, err error) {
	file, err := os.Open(kdbxFile)
	if err != nil {
		return
	}
	db = gokeepasslib.NewDatabase()
	db.Credentials = gokeepasslib.NewPasswordCredentials(password)
	err = gokeepasslib.NewDecoder(file).Decode(db)
	if err != nil {
		return
	}

	err = db.UnlockProtectedEntries()
	if err != nil {
		return
	}
	return db, nil
}

func main() {
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

	flag.Parse()
	if flag.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s MOUNTPOINT KDBXFile\n", os.Args[0])
		os.Exit(2)
	}

	fmt.Println("Enter Password:")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}

	_, err = OpenKDBX(flag.Arg(1), string(pass))
	if err != nil {
		slog.Error("Can't open kdbx file", "error", err)
		os.Exit(1)
	}
}
