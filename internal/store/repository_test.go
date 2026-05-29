package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryStoreCRUD(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".deployctl"), 0755); err != nil {
		t.Fatalf("create deployctl directory: %v", err)
	}

	ctx := context.Background()
	store := NewRepositoryStore()

	repository := Repository{
		Name:        "api",
		URL:         "https://example.test/api.git",
		Location:    filepath.Join(t.TempDir(), "api"),
		ComposePath: "compose.yml",
		EnvPath:     ".env",
	}
	if err := store.Insert(ctx, repository); err != nil {
		t.Fatalf("insert repository: %v", err)
	}

	got, err := store.Get(ctx, "api")
	if err != nil {
		t.Fatalf("get repository: %v", err)
	}
	if got != repository {
		t.Fatalf("repository = %+v, want %+v", got, repository)
	}

	repository.EnvPath = "production.env"
	if err := store.Update(ctx, repository); err != nil {
		t.Fatalf("update repository: %v", err)
	}

	repositories, err := store.GetAll(ctx)
	if err != nil {
		t.Fatalf("get all repositories: %v", err)
	}
	if len(repositories) != 1 || repositories[0].EnvPath != "production.env" {
		t.Fatalf("repositories = %+v", repositories)
	}

	if err := store.Delete(ctx, "api"); err != nil {
		t.Fatalf("delete repository: %v", err)
	}
	if _, err := store.Get(ctx, "api"); err != sql.ErrNoRows {
		t.Fatalf("get deleted repository error = %v, want sql.ErrNoRows", err)
	}
}
