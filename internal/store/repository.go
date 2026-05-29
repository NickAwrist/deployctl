package store

import (
	"context"
	"database/sql"
	"strings"
)

type Repository struct {
	Name        string
	URL         string
	Location    string
	ComposePath string
	EnvPath     string
}

type RepositoryStore struct {
	storage storage
}

func NewRepositoryStore() *RepositoryStore {
	return &RepositoryStore{
		storage: newStorage(),
	}
}

func (s *RepositoryStore) openDatabase() (*sql.DB, error) {
	db, err := s.storage.open()
	if err != nil {
		return nil, err
	}

	if err := migrateRepositories(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func (s *RepositoryStore) Insert(ctx context.Context, repository Repository) error {
	db, err := s.openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.ExecContext(
		ctx,
		"INSERT INTO repositories (name, url, location, compose_path, env_path) VALUES (?, ?, ?, ?, ?)",
		repository.Name,
		repository.URL,
		repository.Location,
		repository.ComposePath,
		repository.EnvPath,
	)
	return err
}

func (s *RepositoryStore) Update(ctx context.Context, repository Repository) error {
	db, err := s.openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()

	result, err := db.ExecContext(
		ctx,
		"UPDATE repositories SET url = ?, location = ?, compose_path = ?, env_path = ? WHERE name = ?",
		repository.URL,
		repository.Location,
		repository.ComposePath,
		repository.EnvPath,
		repository.Name,
	)
	if err != nil {
		return err
	}

	return requireRowsAffected(result)
}

func (s *RepositoryStore) Delete(ctx context.Context, name string) error {
	db, err := s.openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()

	result, err := db.ExecContext(ctx, "DELETE FROM repositories WHERE name = ?", name)
	if err != nil {
		return err
	}

	return requireRowsAffected(result)
}

func (s *RepositoryStore) Get(ctx context.Context, name string) (Repository, error) {
	db, err := s.openDatabase()
	if err != nil {
		return Repository{}, err
	}
	defer db.Close()

	var repository Repository
	err = db.QueryRowContext(
		ctx,
		"SELECT name, url, location, compose_path, env_path FROM repositories WHERE name = ?",
		name,
	).Scan(&repository.Name, &repository.URL, &repository.Location, &repository.ComposePath, &repository.EnvPath)
	if err != nil {
		return Repository{}, err
	}

	return repository, nil
}

func (s *RepositoryStore) GetAll(ctx context.Context) ([]Repository, error) {
	db, err := s.openDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SELECT name, url, location, compose_path, env_path FROM repositories ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repositories []Repository
	for rows.Next() {
		var repository Repository
		if err := rows.Scan(&repository.Name, &repository.URL, &repository.Location, &repository.ComposePath, &repository.EnvPath); err != nil {
			return nil, err
		}
		repositories = append(repositories, repository)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return repositories, nil
}

func (s *RepositoryStore) List(ctx context.Context) ([]Repository, error) {
	db, err := s.openDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SELECT name, url, location, compose_path, env_path FROM repositories ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repositories []Repository
	for rows.Next() {
		var repository Repository
		if err := rows.Scan(&repository.Name, &repository.URL, &repository.Location, &repository.ComposePath, &repository.EnvPath); err != nil {
			return nil, err
		}
		repositories = append(repositories, repository)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return repositories, nil
}

func migrateRepositories(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS repositories (
			name TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			location TEXT NOT NULL,
			compose_path TEXT NOT NULL DEFAULT '',
			env_path TEXT NOT NULL DEFAULT ''
		)
	`); err != nil {
		return err
	}

	for _, statement := range []string{
		`ALTER TABLE repositories ADD COLUMN compose_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE repositories ADD COLUMN env_path TEXT NOT NULL DEFAULT ''`,
	} {
		_, err := db.Exec(statement)
		if err != nil && !isDuplicateColumnError(err) {
			return err
		}
	}

	return nil
}

func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name:")
}

func requireRowsAffected(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
