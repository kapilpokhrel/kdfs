// Package kdbx implements simple wrapper around a gokeepasslib kdbx database
package kdbx

import (
	"os"

	"github.com/tobischo/gokeepasslib/v3"
)

type Database struct {
	db *gokeepasslib.Database
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

	return &Database{db: db}, nil
}

func (d *Database) Root() *gokeepasslib.RootData {
	return d.db.Content.Root
}

func (d *Database) Unlock() error {
	return d.db.UnlockProtectedEntries()
}

func (d *Database) Lock() {
	d.db.LockProtectedEntries()
}

func (d *Database) Save(savepath string) error {
	file, err := os.Create(savepath)
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
