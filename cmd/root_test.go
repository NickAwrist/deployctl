package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"deployctl/internal"
	deployclient "deployctl/internal/client"
	"deployctl/internal/envfile"
	"deployctl/internal/service"
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

func TestVersionFlagPrintsGitCommitBuild(t *testing.T) {
	output, err := executeRoot(t, []string{"--version"}, "")
	if err != nil {
		t.Fatalf("execute version command: %v", err)
	}

	got := strings.TrimSpace(output)
	if !strings.HasPrefix(got, "deployctl build: git commit ") {
		t.Fatalf("version output = %q", got)
	}
	if strings.TrimPrefix(got, "deployctl build: git commit ") == "" {
		t.Fatalf("version output is missing git commit: %q", got)
	}
}

func TestDaemonStatusShowsDaemonAndDockerSections(t *testing.T) {
	setupTestHome(t)

	output, err := executeRoot(t, []string{"daemon", "status"}, "")
	if err != nil {
		t.Fatalf("daemon status command: %v", err)
	}

	for _, want := range []string{"Daemon", "  Status: reachable", "  Socket:", "Docker", "  Status:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("daemon status output %q does not contain %q", output, want)
		}
	}
}

func TestDaemonRestartUsesUserSystemdService(t *testing.T) {
	binDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "restart-called")
	systemctl := filepath.Join(binDir, "systemctl")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--user" ] && [ "$2" = "show" ]; then
  echo loaded
  exit 0
fi
if [ "$1" = "--user" ] && [ "$2" = "restart" ]; then
  touch %q
  exit 0
fi
exit 1
`, marker)
	if err := os.WriteFile(systemctl, []byte(script), 0755); err != nil {
		t.Fatalf("write fake systemctl: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	output, err := executeRoot(t, []string{"daemon", "restart", "--user"}, "")
	if err != nil {
		t.Fatalf("daemon restart command: %v", err)
	}
	if !strings.Contains(output, "deployctld restart requested via systemd user service") {
		t.Fatalf("daemon restart output = %q", output)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("fake systemctl restart was not called: %v", err)
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

	output, err := executeRoot(t, []string{"list"}, "")
	if err != nil {
		t.Fatalf("list command: %v", err)
	}

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

	output, err := executeRoot(t, []string{"env", "list", "api"}, "")
	if err != nil {
		t.Fatalf("env list command: %v", err)
	}
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

func TestEnvSetCopiesEnvFile(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: location})

	envPath := filepath.Join(t.TempDir(), ".env")
	envContents := "# production\nFOO=bar\nBAZ='qux'\n"
	if err := os.WriteFile(envPath, []byte(envContents), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	if _, err := executeRoot(t, []string{"env", "set", "api", envPath}, ""); err != nil {
		t.Fatalf("env set file command: %v", err)
	}

	repository, err := store.NewRepositoryStore().Get(context.Background(), "api")
	if err != nil {
		t.Fatalf("get repository: %v", err)
	}
	if repository.EnvPath != filepath.Join(location, ".env") {
		t.Fatalf("env path = %q", repository.EnvPath)
	}

	got, err := os.ReadFile(repository.EnvPath)
	if err != nil {
		t.Fatalf("read copied env file: %v", err)
	}
	if string(got) != envContents {
		t.Fatalf("copied env file = %q, want %q", got, envContents)
	}
}

func TestEnvSetUpdatesExplicitComposeEnvFile(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	insertRepository(t, store.Repository{
		Name:        "api",
		URL:         "https://example.test/api.git",
		Location:    location,
		ComposePath: filepath.Join(location, "compose.yml"),
	})

	if _, err := executeRoot(t, []string{"env", "set", "api", "app.env", "FOO=bar", "BAZ=qux"}, ""); err != nil {
		t.Fatalf("env set explicit file command: %v", err)
	}

	variables, err := envfile.Read(filepath.Join(location, "app.env"))
	if err != nil {
		t.Fatalf("read app env file: %v", err)
	}
	if variables["FOO"] != "bar" || variables["BAZ"] != "qux" {
		t.Fatalf("variables after set = %#v", variables)
	}

	repository, err := store.NewRepositoryStore().Get(context.Background(), "api")
	if err != nil {
		t.Fatalf("get repository: %v", err)
	}
	if repository.EnvPath != "" {
		t.Fatalf("default env path = %q, want empty", repository.EnvPath)
	}
}

func TestEnvSetCopiesExplicitComposeEnvFile(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	insertRepository(t, store.Repository{
		Name:        "api",
		URL:         "https://example.test/api.git",
		Location:    location,
		ComposePath: filepath.Join(location, "compose.yml"),
	})

	source := filepath.Join(t.TempDir(), "backend.env")
	contents := "DATABASE_URL=postgres://example\n"
	if err := os.WriteFile(source, []byte(contents), 0600); err != nil {
		t.Fatalf("write source env file: %v", err)
	}

	if _, err := executeRoot(t, []string{"env", "set", "api", "backend.env", source}, ""); err != nil {
		t.Fatalf("env set explicit file copy command: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(location, "backend.env"))
	if err != nil {
		t.Fatalf("read backend env file: %v", err)
	}
	if string(got) != contents {
		t.Fatalf("copied env file = %q, want %q", got, contents)
	}
}

func TestEnvListAndUnsetUseExplicitComposeEnvFile(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: location})

	if _, err := executeRoot(t, []string{"env", "set", "api", "app.env", "FOO=bar", "BAZ=qux"}, ""); err != nil {
		t.Fatalf("env set explicit file command: %v", err)
	}

	output, err := executeRoot(t, []string{"env", "list", "api", "app.env"}, "")
	if err != nil {
		t.Fatalf("env list explicit file command: %v", err)
	}
	if !strings.Contains(output, "BAZ=*****") || !strings.Contains(output, "FOO=*****") {
		t.Fatalf("env list output = %q", output)
	}

	if _, err := executeRoot(t, []string{"env", "unset", "api", "app.env", "FOO"}, ""); err != nil {
		t.Fatalf("env unset explicit file command: %v", err)
	}

	variables, err := envfile.Read(filepath.Join(location, "app.env"))
	if err != nil {
		t.Fatalf("read app env file: %v", err)
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

func TestDeploymentCommandsReportMissingComposeFile(t *testing.T) {
	setupTestHome(t)
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: t.TempDir()})

	for _, args := range [][]string{
		{"deploy", "api"},
		{"stop", "api"},
		{"restart", "api"},
		{"restart", "api", "--build"},
	} {
		_, err := executeRoot(t, args, "")
		if err == nil || !strings.Contains(err.Error(), "compose file") {
			t.Fatalf("%v error = %v, want missing compose file", args, err)
		}
	}
}

func TestConfirmBuild(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  bool
	}{
		{name: "default yes", input: "\n", want: true},
		{name: "yes", input: "yes\n", want: true},
		{name: "no", input: "n\n", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got bool
			var err error
			captureStdout(t, func() {
				got, err = confirmBuild(strings.NewReader(tc.input), []string{"api-web"})
			})
			if err != nil {
				t.Fatalf("confirm build: %v", err)
			}
			if got != tc.want {
				t.Fatalf("confirm build = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestConfirmDelete(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  bool
	}{
		{name: "lowercase y", input: "y\n", want: true},
		{name: "uppercase y", input: "Y\n", want: true},
		{name: "yes", input: "yes\n", want: true},
		{name: "mixed case yes", input: "YeS\n", want: true},
		{name: "default no", input: "\n", want: false},
		{name: "no", input: "n\n", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got bool
			var err error
			captureStdout(t, func() {
				got, err = confirmDelete(strings.NewReader(tc.input), io.Discard, "api", false)
			})
			if err != nil {
				t.Fatalf("confirm delete: %v", err)
			}
			if got != tc.want {
				t.Fatalf("confirm delete = %v, want %v", got, tc.want)
			}
		})
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

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("dct-%d-%d.sock", os.Getpid(), time.Now().UnixNano()))
	t.Setenv("DEPLOYCTL_SOCKET_PATH", socketPath)
	t.Cleanup(func() {
		_ = os.Remove(socketPath)
	})
	internal.InitializeDirectoryStructure()
	startTestDaemon(t)
}

func startTestDaemon(t *testing.T) {
	t.Helper()

	listener, err := service.ListenUnix(internal.GetSocketPath())
	if err != nil {
		t.Fatalf("listen test daemon: %v", err)
	}
	grpcServer := service.NewGRPCServer(service.NewServer())
	t.Cleanup(grpcServer.Stop)
	go func() {
		_ = grpcServer.Serve(listener)
	}()

	client, err := deployclient.DialDefault(context.Background())
	if err != nil {
		t.Fatalf("dial test daemon: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
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
