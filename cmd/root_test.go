package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"deployctl/internal"
	"deployctl/internal/envfile"
	"deployctl/internal/store"
)

func TestRootCommandPrintsName(t *testing.T) {
	output, err := executeRoot(t, nil, "")
	if err != nil {
		t.Fatalf("execute root command: %v", err)
	}

	if got := strings.TrimSpace(output); got != "deployctl" {
		t.Fatalf("root output = %q, want deployctl", got)
	}
}

func TestCreateCommandClonesRepoAndStoresDeployment(t *testing.T) {
	setupTestHome(t)
	sourceRepo := createGitRepository(t, map[string]string{
		"compose.yml": ".services: {}\n",
		"app.env":     "PORT=8080\nDEBUG=true\n",
	})

	if _, err := executeRoot(t, []string{"create", sourceRepo, "--name", "api", "--env-file", "app.env"}, ""); err != nil {
		t.Fatalf("create command: %v", err)
	}

	repository, err := store.NewRepositoryStore().Get(context.Background(), "api")
	if err != nil {
		t.Fatalf("get repository: %v", err)
	}

	wantLocation := filepath.Join(internal.GetRepositoryDirectory(), "api")
	if repository.URL != sourceRepo || repository.Location != wantLocation {
		t.Fatalf("stored repository = %+v", repository)
	}
	if repository.ComposePath != filepath.Join(wantLocation, "compose.yml") {
		t.Fatalf("compose path = %q", repository.ComposePath)
	}
	if repository.EnvPath != filepath.Join(wantLocation, ".env") {
		t.Fatalf("env path = %q", repository.EnvPath)
	}

	variables, err := envfile.Read(repository.EnvPath)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	if variables["PORT"] != "8080" || variables["DEBUG"] != "true" {
		t.Fatalf("env variables = %#v", variables)
	}
}

func TestListCommandShowsDeployments(t *testing.T) {
	setupTestHome(t)
	insertRepository(t, store.Repository{
		Name:        "api",
		URL:         "https://example.test/api.git",
		Location:    "/tmp/api",
		ComposePath: "/tmp/api/compose.yml",
		EnvPath:     "/tmp/api/.env",
	})

	output := captureStdout(t, func() {
		if _, err := executeRoot(t, []string{"list"}, ""); err != nil {
			t.Fatalf("list command: %v", err)
		}
	})

	for _, want := range []string{"api:", "https://example.test/api.git", "/tmp/api/compose.yml", "/tmp/api/.env"} {
		if !strings.Contains(output, want) {
			t.Fatalf("list output %q does not contain %q", output, want)
		}
	}
}

func TestEnvCommandsSetListAndUnsetVariables(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: location})

	if _, err := executeRoot(t, []string{"env", "set", "api", "FOO=bar", "BAZ=qux"}, ""); err != nil {
		t.Fatalf("env set command: %v", err)
	}

	repository, err := store.NewRepositoryStore().Get(context.Background(), "api")
	if err != nil {
		t.Fatalf("get repository: %v", err)
	}
	if repository.EnvPath != filepath.Join(location, ".env") {
		t.Fatalf("env path = %q", repository.EnvPath)
	}

	variables, err := envfile.Read(repository.EnvPath)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	if variables["FOO"] != "bar" || variables["BAZ"] != "qux" {
		t.Fatalf("variables after set = %#v", variables)
	}

	output := captureStdout(t, func() {
		if _, err := executeRoot(t, []string{"env", "list", "api"}, ""); err != nil {
			t.Fatalf("env list command: %v", err)
		}
	})
	if !strings.Contains(output, "BAZ=*****") || !strings.Contains(output, "FOO=*****") {
		t.Fatalf("env list output = %q", output)
	}
	if strings.Contains(output, "bar") || strings.Contains(output, "qux") {
		t.Fatalf("env list leaked values: %q", output)
	}

	if _, err := executeRoot(t, []string{"env", "unset", "api", "FOO"}, ""); err != nil {
		t.Fatalf("env unset command: %v", err)
	}

	variables, err = envfile.Read(repository.EnvPath)
	if err != nil {
		t.Fatalf("read env file after unset: %v", err)
	}
	if _, ok := variables["FOO"]; ok || variables["BAZ"] != "qux" {
		t.Fatalf("variables after unset = %#v", variables)
	}
}

func TestDeleteCommandCancelsAndForceDeletesDeployment(t *testing.T) {
	setupTestHome(t)
	location := filepath.Join(internal.GetRepositoryDirectory(), "api")
	if err := os.MkdirAll(location, 0755); err != nil {
		t.Fatalf("create repository directory: %v", err)
	}
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: location})

	if _, err := executeRoot(t, []string{"delete", "api"}, "n\n"); err != nil {
		t.Fatalf("delete cancel command: %v", err)
	}
	if _, err := os.Stat(location); err != nil {
		t.Fatalf("repository directory after cancel: %v", err)
	}

	if _, err := executeRoot(t, []string{"delete", "api", "--force"}, ""); err != nil {
		t.Fatalf("delete force command: %v", err)
	}
	if _, err := os.Stat(location); !os.IsNotExist(err) {
		t.Fatalf("repository directory still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := store.NewRepositoryStore().Get(context.Background(), "api"); err != sql.ErrNoRows {
		t.Fatalf("repository lookup after delete error = %v, want sql.ErrNoRows", err)
	}
}

func TestDeployAndStopReportMissingComposeFile(t *testing.T) {
	setupTestHome(t)
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: t.TempDir()})

	for _, args := range [][]string{
		{"deploy", "api"},
		{"stop", "api"},
	} {
		_, err := executeRoot(t, args, "")
		if err == nil || !strings.Contains(err.Error(), "compose file") {
			t.Fatalf("%v error = %v, want missing compose file", args, err)
		}
	}
}

func TestCompleteDeploymentNamesFiltersByPrefix(t *testing.T) {
	setupTestHome(t)
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: "/tmp/api"})
	insertRepository(t, store.Repository{Name: "worker", URL: "https://example.test/worker.git", Location: "/tmp/worker"})

	matches, directive := completeDeploymentNames(rootCmd, nil, "a")
	if directive == 0 {
		t.Fatal("completion directive should disable file completion")
	}
	if len(matches) != 1 || matches[0] != "api" {
		t.Fatalf("matches = %#v, want api", matches)
	}
}

func executeRoot(t *testing.T, args []string, input string) (string, error) {
	t.Helper()

	var output bytes.Buffer
	rootCmd.SetArgs(args)
	rootCmd.SetIn(strings.NewReader(input))
	rootCmd.SetOut(&output)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	return output.String(), err
}

func setupTestHome(t *testing.T) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	internal.InitializeDirectoryStructure()
}

func insertRepository(t *testing.T, repository store.Repository) {
	t.Helper()

	if err := store.NewRepositoryStore().Insert(context.Background(), repository); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
}

func createGitRepository(t *testing.T, files map[string]string) string {
	t.Helper()

	directory := t.TempDir()
	for name, content := range files {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	runGit(t, directory, "init")
	runGit(t, directory, "add", ".")
	runGit(t, directory, "-c", "user.name=deployctl", "-c", "user.email=deployctl@example.test", "commit", "-m", "initial")

	return directory
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	os.Stdout = original

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	return string(output)
}
