package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kapilpokhrel/kdfs/internal/kdfs"
	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

func TestMountDelete(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "testdb.kdbx")
	password := []byte("testpassword")

	db := createNewKdbxDB(dbPath, password)

	testGroup := gokeepasslib.NewGroup()
	testGroup.Name = "TestGroup"

	testEntry := gokeepasslib.NewEntry()
	testEntry.Values = append(
		testEntry.Values,
		gokeepasslib.ValueData{Key: "Title", Value: gokeepasslib.V{Content: "TestEntry", Protected: wrappers.BoolWrapper{Bool: true}}},
	)
	db.Root().Groups[0].Groups = append(db.Root().Groups[0].Groups, testGroup)
	db.Root().Groups[0].Entries = append(db.Root().Groups[0].Entries, testEntry)
	db.Save()

	mountDir := t.TempDir()

	serverCfg := kdfs.KDFSConfig{KDBXValutPath: dbPath, MountPoint: mountDir}
	server, err := kdfs.NewKDFSServer(serverCfg, password)
	if err != nil {
		t.Fatalf("mount failed: %v", err)
	}
	defer server.Umount()

	path := filepath.Join(mountDir, "RootGroup", "TestGroup")
	if err := os.Remove(path); err != nil {
		t.Fatalf("Couldn't remove the group %s %v", path, err)
	}
	path = filepath.Join(mountDir, "RootGroup", "TestEntry.entry")
	if err := os.Remove(path); err != nil {
		t.Fatalf("Couldn't remove the entry %s %v", path, err)
	}

	if len(server.DB.Root().Groups[0].Groups) != 0 {
		t.Fatalf("Group not deleted in gokeepass DB")
	}

	if len(server.DB.Root().Groups[0].Entries) != 0 {
		t.Fatalf("entry not deleted in gokeepass DB")
	}
}
