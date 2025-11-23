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

func splitPath(p string) []string {
	pathSplit := strings.Split(p, "/")
	if len(pathSplit) > 0 && pathSplit[0] == "" {
		pathSplit = pathSplit[1:] // Remove the root / if present
	}
	return pathSplit
}

func findChildGroup(groups []gokeepasslib.Group, name string) (*gokeepasslib.Group, error) {
	for i, group := range groups {
		if group.Name == name {
			return &groups[i], nil
		}
	}
	return nil, ErrNotFound
}

func findEntryInGroup(base *gokeepasslib.Group, title string) (*gokeepasslib.Entry, error) {
	for i := range base.Entries {
		if base.Entries[i].GetTitle() == title {
			return &base.Entries[i], nil
		}
	}
	return nil, ErrNotFound
}

var groupCache = make(map[string]*gokeepasslib.Group)

func (d *Database) findGroup(baseGroup *gokeepasslib.Group, groupPath string) (*gokeepasslib.Group, error) {
	cachedGroup, ok := groupCache[groupPath]
	if ok {
		return cachedGroup, nil
	}

	pathSplit := splitPath(groupPath)
	if len(pathSplit) == 0 {
		return nil, ErrNotFound
	}

	if baseGroup == nil {
		var err error
		baseGroup, err = findChildGroup(d.Root().Groups, pathSplit[0])
		if err != nil {
			return nil, err
		}
		pathSplit = pathSplit[1:]
	}

	for len(pathSplit) > 0 {
		next, err := findChildGroup(baseGroup.Groups, pathSplit[0])
		if err != nil {
			return nil, err
		}
		baseGroup = next
		pathSplit = pathSplit[1:]
	}

	groupCache[groupPath] = baseGroup
	return baseGroup, nil
}

var entryCache = make(map[string]*gokeepasslib.Entry)

func (d *Database) findEntry(baseGroup *gokeepasslib.Group, entryPath string) (*gokeepasslib.Entry, error) {
	cachedEntry, ok := entryCache[entryPath]
	if ok {
		return cachedEntry, nil
	}

	pathSplit := splitPath(entryPath)
	if len(pathSplit) == 0 {
		return nil, ErrNotFound
	}

	entryName := pathSplit[len(pathSplit)-1]
	groupPath := pathSplit[:len(pathSplit)-1]

	var err error
	baseGroup, err = d.findGroup(baseGroup, strings.Join(groupPath, "/"))
	if err != nil {
		return nil, ErrNotFound
	}

	entry, err := findEntryInGroup(baseGroup, entryName)
	if err != nil {
		return nil, err
	}

	entryCache[entryPath] = entry
	return entry, nil
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
