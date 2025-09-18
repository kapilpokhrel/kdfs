package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kapilpokhrel/kdfs/internal/kdfs"
	"github.com/tobischo/gokeepasslib/v3"
)

func collectExpectedFiles(g gokeepasslib.Group, base string, paths *map[string]string) {
	for _, entry := range g.Entries {
		title := entry.GetTitle()
		if len(title) == 0 {
			continue
		}
		entryBase := filepath.Join(base, entry.GetTitle())
		for _, val := range entry.Values {
			switch val.Key {
			case "UserName":
				if val.Value.Content != "" {
					(*paths)[filepath.Join(entryBase, "username")] = val.Value.Content
				}
			case "Password":
				if val.Value.Content != "" {
					(*paths)[filepath.Join(entryBase, "password")] = val.Value.Content
				}
			case "Notes":
				if val.Value.Content != "" {
					(*paths)[filepath.Join(entryBase, "notes")] = val.Value.Content
				}
			case "URL":
				if val.Value.Content != "" {
					(*paths)[filepath.Join(entryBase, "url")] = val.Value.Content
				}
			}
		}
	}

	for _, subgroup := range g.Groups {
		subdir := filepath.Join(base, subgroup.Name)
		(*paths)[subdir] = ""
		collectExpectedFiles(subgroup, subdir, paths)
	}
}

func TestMountRONLY(t *testing.T) {
	kdbxFile := "./_datafiles/example.kdbx"
	password := []byte("abcdefg12345678")

	mountDir := t.TempDir()

	server, err := kdfs.NewKDFSServer(kdbxFile, password, mountDir)
	if err != nil {
		t.Fatalf("failed to create kdfs server, %v", err)
	}
	defer server.Umount()

	expected := make(map[string]string)

	rootGroup := server.DB.Root().Groups[0]
	collectExpectedFiles(rootGroup, filepath.Join(mountDir, rootGroup.Name), &expected)

	found := make(map[string]struct{})
	err = filepath.Walk(mountDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path != mountDir {
			found[path] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk FS: %v", err)
	}

	for path, content := range expected {
		if _, ok := found[path]; !ok {
			t.Errorf("expected path not found in FS: %s", path)
			continue
		}
		if content != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("failed to read file %s: %v", path, err)
				continue
			}
			if string(data) != content {
				t.Errorf("file %s content mismatch. got: %q, want: %q", path, string(data), content)
			}
		}
	}
}
