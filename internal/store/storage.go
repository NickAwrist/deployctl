package store

import (
	"database/sql"
	"path/filepath"

	"deployctl/internal"

	_ "modernc.org/sqlite"
)

const databaseFileName = "deployctl.db"

type storage struct {
	path string
}

func newStorage() storage {
	return storage{
		path: filepath.Join(internal.GetMainDirectory(), databaseFileName),
	}
}

func (s storage) open() (*sql.DB, error) {
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
