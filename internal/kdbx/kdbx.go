// Package kdbx implements simple wrapper around a gokeepasslib kdbx database
package kdbx

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/tobischo/gokeepasslib/v3"
)

var (
	ErrNotFound     = fmt.Errorf("not found")
	ErrAlreadyExist = fmt.Errorf("already exists")
)

type Database struct {
	db     *gokeepasslib.Database
	path   string
	locked bool
	mu     sync.RWMutex
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

	return &Database{db: db, path: path, locked: true}, nil
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
	d.mu.RLock()
	defer d.mu.RUnlock()

	baseEntry, err := d.findEntry(baseG, entryPath)
	if err != nil {
		return
	}
	return baseEntry.Clone(), nil
}

func (d *Database) SetEntry(baseG *gokeepasslib.Group, entryPath string, entry gokeepasslib.Entry) (err error) {
	d.mu.Lock()

	baseEntry, err := d.findEntry(baseG, entryPath)
	if err != nil {
		return
	}
	*baseEntry = entry
	d.mu.Unlock()
	return d.Save()
}

func (d *Database) GetState() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.locked
}

func (d *Database) GetGroup(baseG *gokeepasslib.Group, groupPath string) (group gokeepasslib.Group, err error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	baseGroup, err := d.findGroup(baseG, groupPath)
	if err != nil {
		return
	}
	return baseGroup.Clone(), nil
}

func (d *Database) SetGroup(baseG *gokeepasslib.Group, groupPath string, group gokeepasslib.Group) (err error) {
	d.mu.Lock()

	/* Updates the old group if present else add it */
	baseGroup, err := d.findGroup(baseG, groupPath)
	if err != nil {
		return
	}
	*baseGroup = group
	d.mu.Unlock()
	return d.Save()
}

func (d *Database) Unlock() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.locked {
		return ErrAlreadyExist
	}
	err := d.db.UnlockProtectedEntries()
	if err != nil {
		return err
	}
	d.locked = false
	return err
}

func (d *Database) Lock() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.locked {
		return ErrAlreadyExist
	}
	err := d.db.LockProtectedEntries()
	if err != nil {
		return err
	}
	d.locked = true
	return nil
}

func (d *Database) Save() error {
	err := d.Lock()
	if err == nil {
		defer d.Unlock()
	} else if !errors.Is(err, ErrAlreadyExist) {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

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
