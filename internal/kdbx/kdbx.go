// Package kdbx implements simple wrapper around a gokeepasslib kdbx database
package kdbx

import (
	"fmt"
	"os"
	"strings"

	"github.com/tobischo/gokeepasslib/v3"
)

var ErrNotFound = fmt.Errorf("not found")

type Database struct {
	db   *gokeepasslib.Database
	path string
}

func Open(path string, password []byte) (*Database, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	db := gokeepasslib.NewDatabase()
	db.Credentials = gokeepasslib.NewPasswordCredentials(string(password))

	if err := gokeepasslib.NewDecoder(file).Decode(db); err != nil {
		return nil, err
	}

	return &Database{db: db, path: path}, nil
}

func (d *Database) Root() *gokeepasslib.RootData {
	return d.db.Content.Root
}

func (d *Database) findEntry(baseG *gokeepasslib.Group, entryPath string) (*gokeepasslib.Entry, error) {
	pathSplit := strings.Split(entryPath, "/")
	if pathSplit[0] == "" {
		pathSplit = pathSplit[1:] // Remove the root / if present
	}
	if baseG == nil {
		for i, group := range d.Root().Groups {
			if group.Name == pathSplit[0] {
				baseG = &d.Root().Groups[i]
				pathSplit = pathSplit[1:]
				break
			}
		}
	}
	for range len(pathSplit) {
		if len(pathSplit) == 1 {
			// Entry
			for i, entry := range baseG.Entries {
				if entry.GetTitle() == pathSplit[0] {
					return &baseG.Entries[i], nil
				}
			}
		}
		// Group
		for i, group := range baseG.Groups {
			if group.Name == pathSplit[0] {
				baseG = &baseG.Groups[i]
				break
			}
		}
		pathSplit = pathSplit[1:]

	}
	return nil, ErrNotFound
}

func (d *Database) findGroup(baseG *gokeepasslib.Group, entryPath string) (*gokeepasslib.Group, error) {
	pathSplit := strings.Split(entryPath, "/")
	if pathSplit[0] == "" {
		pathSplit = pathSplit[1:] // Remove the root / if present
	}
	if baseG == nil {
		for i, group := range d.Root().Groups {
			if group.Name == pathSplit[0] {
				baseG = &d.Root().Groups[i]
				pathSplit = pathSplit[1:]
				break
			}
		}
	}

	if baseG == nil {
		return nil, ErrNotFound
	}

	for range len(pathSplit) {
		notfound := true
		for i, group := range baseG.Groups {
			if group.Name == pathSplit[0] {
				baseG = &baseG.Groups[i]
				notfound = false
				break
			}
		}
		if notfound {
			return nil, ErrNotFound
		}
		pathSplit = pathSplit[1:]
	}
	return baseG, nil
}

func (d *Database) GetEntry(baseG *gokeepasslib.Group, entryPath string) (entry gokeepasslib.Entry, err error) {
	baseEntry, err := d.findEntry(baseG, entryPath)
	if err != nil {
		return
	}
	return baseEntry.Clone(), nil
}

func (d *Database) SetEntry(baseG *gokeepasslib.Group, entryPath string, entry gokeepasslib.Entry) (err error) {
	baseEntry, err := d.findEntry(baseG, entryPath)
	if err != nil {
		return
	}
	*baseEntry = entry
	return d.Save()
}

func (d *Database) GetGroup(baseG *gokeepasslib.Group, groupPath string) (group gokeepasslib.Group, err error) {
	baseGroup, err := d.findGroup(baseG, groupPath)
	if err != nil {
		return
	}
	return baseGroup.Clone(), nil
}

func (d *Database) SetGroup(baseG *gokeepasslib.Group, groupPath string, group gokeepasslib.Group) (err error) {
	/* Updates the old group if present else add it */
	baseGroup, err := d.findGroup(baseG, groupPath)
	if err != nil {
		return
	}
	*baseGroup = group
	return d.Save()
}

func (d *Database) Unlock() error {
	return d.db.UnlockProtectedEntries()
}

func (d *Database) Lock() {
	d.db.LockProtectedEntries()
}

func (d *Database) Save() error {
	d.Lock()
	defer d.Unlock()

	file, err := os.Create(d.path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gokeepasslib.NewEncoder(file)
	return encoder.Encode(d.db)
}

func (d *Database) Raw() *gokeepasslib.Database {
	return d.db
}
