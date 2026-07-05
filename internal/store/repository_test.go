package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryStoreCRUD(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, ".deployctl"), 0755); err != nil {
		t.Fatalf("create deployctl directory: %v", err)
	}

	ctx := context.Background()
	dataStore, err := OpenDefault()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = dataStore.Close()
	})
	repositories := dataStore.Repositories

	repository := Repository{
		Name:        "api",
		URL:         "https://example.test/api.git",
		Location:    filepath.Join(t.TempDir(), "api"),
		ComposePath: "compose.yml",
		EnvPath:     ".env",
	}
	if err := repositories.Insert(ctx, repository); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := repositories.Insert(ctx, repository); !IsConflict(err) {
		t.Fatalf("duplicate repository insert error = %v, want conflict", err)
	}

	got, err := repositories.Get(ctx, "api")
	if err != nil {
		t.Fatalf("get repository: %v", err)
	}
	if got != repository {
		t.Fatalf("repository = %+v, want %+v", got, repository)
	}

	repository.EnvPath = "production.env"
	if err := repositories.Update(ctx, repository); err != nil {
		t.Fatalf("update repository: %v", err)
	}

	gotRepositories, err := repositories.GetAll(ctx)
	if err != nil {
		t.Fatalf("get all repositories: %v", err)
	}
	if len(gotRepositories) != 1 || gotRepositories[0].EnvPath != "production.env" {
		t.Fatalf("repositories = %+v", gotRepositories)
	}

	if err := repositories.Delete(ctx, "api"); err != nil {
		t.Fatalf("delete repository: %v", err)
	}
	if _, err := repositories.Get(ctx, "api"); !IsNotFound(err) {
		t.Fatalf("get deleted repository error = %v, want not found", err)
	}
}
