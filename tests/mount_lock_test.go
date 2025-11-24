package tests

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kapilpokhrel/kdfs/internal/kdfs"
)

func readLockState(rootDir string) (bool, error) {
	lockStateFile := filepath.Join(rootDir, "lockstate")
	data, err := os.ReadFile(lockStateFile)
	if err != nil {
		return false, errors.Join(err, errors.New("failed to read lockstate file"))
	}
	switch string(data) {
	case "t":
		return true, nil
	case "f":
		return false, nil
	default:
		return false, fmt.Errorf("unexpcted entry in lockfile, %v", string(data))
	}
}

func writeToLockFile(rootDir string, data []byte) error {
	lockActionFile := filepath.Join(rootDir, "lockaction")
	f, err := os.OpenFile(lockActionFile, os.O_WRONLY|os.O_APPEND, 0o200)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func lock(rootDir string) error {
	return writeToLockFile(rootDir, []byte("lock"))
}

func unlock(rootDir string, password []byte) error {
	return writeToLockFile(rootDir, password)
}

func cloneKDBX() (string, error) {
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	src, err := os.Open("_datafiles/example.kdbx")
	if err != nil {
		return "", err
	}
	defer src.Close()

	_, err = io.Copy(tmp, src)
	if err != nil {
		return "", err
	}

	return tmp.Name(), nil
}

func TestMountLockUnlock(t *testing.T) {
	kdbxFile, err := cloneKDBX()
	if err != nil {
		t.Fatalf("error cloning a kdbx file %v", err)
	}
	password := []byte("abcdefg12345678")

	mountDir := t.TempDir()

	server, err := kdfs.NewKDFSServer(kdbxFile, password, mountDir)
	if err != nil {
		t.Fatalf("failed to create kdfs server, %v", err)
	}
	defer server.Umount()

	path := filepath.Join(mountDir, "example", "General", "Sample Entry.entry", "password")

	/* Initial data read */
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file before locking, %s: %v", path, err)
	}
	if string(data) != "Password" {
		t.Fatalf("failed reading password before locking, expected %v, got %v", "Password", string(data))
	}

	/* Initial state read */
	lockstate, err := readLockState(mountDir)
	if err != nil {
		t.Fatalf("before locking, %v", err)
	}
	if lockstate {
		t.Fatalf("before locking, expected unlocked, got locked")
	}

	/* Data + state verify after locking */
	if err := lock(mountDir); err != nil {
		t.Fatalf("locking failed %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file after lock %s: %v", path, err)
	}
	lockedPass := "AsVghNbhsNk="
	if string(data) != lockedPass {
		t.Fatalf("failed reading locked password, expected %v, got %v", lockedPass, string(data))
	}

	lockstate, err = readLockState(mountDir)
	if err != nil {
		t.Fatalf("after locking, %v", err)
	}
	if !lockstate {
		t.Fatalf("after locking, expected locked, got unlocked")
	}

	/* Data + state verify after unlocking */
	if err := unlock(mountDir, password); err != nil {
		t.Fatalf("unlocking failed %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file before locking, %s: %v", path, err)
	}
	if string(data) != "Password" {
		t.Fatalf("failed reading unlocked password, expected %v, got %v", "Password", string(data))
	}

	lockstate, err = readLockState(mountDir)
	if err != nil {
		t.Fatalf("after unlocking, %v", err)
	}
	if lockstate {
		t.Fatalf("after unlocking, expected unlocked, got locked")
	}
}
