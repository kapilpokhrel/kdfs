package tests

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kapilpokhrel/kdfs/internal/kdbx"
	"github.com/kapilpokhrel/kdfs/internal/kdfs"
	"github.com/tobischo/gokeepasslib/v3"
)

func createNewKdbxDB(path string, password []byte) *kdbx.Database {
	db := gokeepasslib.NewDatabase()
	db.Credentials = gokeepasslib.NewPasswordCredentials(string(password))

	root := gokeepasslib.NewGroup()
	root.Name = "RootGroup"
	db.Content.Root = &gokeepasslib.RootData{Groups: []gokeepasslib.Group{root}}
	kdbxDB := kdbx.NewFromDB(db, path, false)
	kdbxDB.Save()
	return kdbxDB
}

func createTestEntry(baseDir string, name string) error {
	entryDir := filepath.Join(baseDir, fmt.Sprintf("%s.entry", name))
	if err := os.MkdirAll(entryDir, 0o755); err != nil {
		return errors.Join(err, errors.New("failed making entry dir"))
	}

	if err := os.WriteFile(filepath.Join(entryDir, "username"), []byte("alice"), 0o600); err != nil {
		return errors.Join(err, errors.New("failed writing username"))
	}

	if err := os.WriteFile(filepath.Join(entryDir, "password"), []byte("s3cret!"), 0o600); err != nil {
		return errors.Join(err, errors.New("failed writing password"))
	}

	return nil
}

func checkTestEntry(baseDir string, name string) error {
	data, err := os.ReadFile(filepath.Join(baseDir, fmt.Sprintf("%s.entry", name), "username"))
	if err != nil {
		errors.Join(err, errors.New("failed to read back username"))
	}
	if string(data) != "alice" {
		errors.Join(err, fmt.Errorf("username mismatch: got %q, want %q", data, "alice"))
	}

	data, err = os.ReadFile(filepath.Join(baseDir, fmt.Sprintf("%s.entry", name), "password"))
	if err != nil {
		errors.Join(err, errors.New("failed to read back password"))
	}
	if string(data) != "s3cret!" {
		errors.Join(err, fmt.Errorf("password mismatch: got %q, want %q", data, "s3cret!"))
	}
	return nil
}

func TestMountWriteThenRead(t *testing.T) {
	/* Writing */
	dbPath := filepath.Join(t.TempDir(), "testdb.kdbx")
	password := []byte("testpassword")

	createNewKdbxDB(dbPath, password)

	mountDir := t.TempDir()
	server, err := kdfs.NewKDFSServer(dbPath, password, mountDir)
	if err != nil {
		t.Fatalf("mount failed: %v", err)
	}

	if err := createTestEntry(filepath.Join(mountDir, "RootGroup", "TestGroup"), "TestEntry"); err != nil {
		t.Fatal(err)
	}
	if err := createTestEntry(filepath.Join(mountDir, "RootGroup"), "TestEntry"); err != nil {
		t.Fatal(err)
	}

	if err := server.Umount(); err != nil {
		t.Fatalf("unmount failed: %v", err)
	}

	/* Verifying the write */
	mountDir2 := t.TempDir()
	server2, err := kdfs.NewKDFSServer(dbPath, password, mountDir2)
	if err != nil {
		t.Fatalf("second mount failed: %v", err)
	}

	if err := checkTestEntry(filepath.Join(mountDir2, "RootGroup"), "TestEntry"); err != nil {
		t.Fatal(err)
	}
	if err := checkTestEntry(filepath.Join(mountDir2, "RootGroup", "TestGroup"), "TestEntry"); err != nil {
		t.Fatal(err)
	}

	if err := server2.Umount(); err != nil {
		t.Fatalf("server2 unmount failed: %v", err)
	}
}
