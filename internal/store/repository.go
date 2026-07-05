package store

import (
	"context"
	"database/sql"
	"errors"
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
	db *sql.DB
}

func (s *RepositoryStore) Insert(ctx context.Context, repository Repository) error {
	_, err := s.db.ExecContext(
		ctx,
		"INSERT INTO repositories (name, url, location, compose_path, env_path) VALUES (?, ?, ?, ?, ?)",
		repository.Name,
		repository.URL,
		repository.Location,
		repository.ComposePath,
		repository.EnvPath,
	)
	if err != nil && isUniqueConstraintError(err) {
		return ErrConflict
	}
	return err
}

func (s *RepositoryStore) Update(ctx context.Context, repository Repository) error {
	result, err := s.db.ExecContext(
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
	result, err := s.db.ExecContext(ctx, "DELETE FROM repositories WHERE name = ?", name)
	if err != nil {
		return err
	}

	return requireRowsAffected(result)
}

func (s *RepositoryStore) Get(ctx context.Context, name string) (Repository, error) {
	var repository Repository
	err := s.db.QueryRowContext(
		ctx,
		"SELECT name, url, location, compose_path, env_path FROM repositories WHERE name = ?",
		name,
	).Scan(&repository.Name, &repository.URL, &repository.Location, &repository.ComposePath, &repository.EnvPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Repository{}, ErrNotFound
		}
		return Repository{}, err
	}

	return repository, nil
}

func (s *RepositoryStore) GetAll(ctx context.Context) ([]Repository, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT name, url, location, compose_path, env_path FROM repositories ORDER BY name")
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

func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed:")
}

func requireRowsAffected(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}
