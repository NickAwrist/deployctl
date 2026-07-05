package store

import (
	"database/sql"
	"path/filepath"

	"deployctl/internal"

	_ "modernc.org/sqlite"
)

const databaseFileName = "deployctl.db"

type Store struct {
	db           *sql.DB
	Repositories *RepositoryStore
	Jobs         *JobStore
}

func OpenDefault() (*Store, error) {
	mainDirectory, err := internal.MainDirectory()
	if err != nil {
		return nil, err
	}
	return Open(filepath.Join(mainDirectory, databaseFileName))
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{
		db:           db,
		Repositories: &RepositoryStore{db: db},
		Jobs:         &JobStore{db: db},
	}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	if err := migrateRepositories(db); err != nil {
		return err
	}
	return migrateJobs(db)
}
