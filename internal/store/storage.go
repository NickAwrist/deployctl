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
	return sql.Open("sqlite", s.path)
}
