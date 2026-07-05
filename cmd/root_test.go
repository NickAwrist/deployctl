package cmd

import (
	"bytes"
	"context"
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
	"deployctl/internal/rpc"
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

	repository, err := getRepository(t, "api")
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
		Name:     "api",
		URL:      "https://example.test/api.git",
		Location: "/tmp/api",
		EnvPath:  "/tmp/api/.env",
	})

	output, err := executeRoot(t, []string{"list"}, "")
	if err != nil {
		t.Fatalf("list command: %v", err)
	}

	for _, want := range []string{"NAME", "STATUS", "REPOSITORY", "api", "not_configured", "https://example.test/api.git", "/tmp/api", "none", "/tmp/api/.env"} {
		if !strings.Contains(output, want) {
			t.Fatalf("list output %q does not contain %q", output, want)
		}
	}
	if strings.Contains(output, "ERROR") {
		t.Fatalf("list output %q should not contain an error column", output)
	}
}

func TestHistoryCommandShowsDeploymentJobs(t *testing.T) {
	setupTestHome(t)
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: "/tmp/api"})
	finishedAt := time.Unix(1_800_000_000, 0)
	insertJob(t, store.Job{
		ID:             "job-1",
		Type:           "update",
		DeploymentName: "api",
		Status:         store.JobStatusFailed,
		CreatedAt:      finishedAt.Add(-time.Minute),
		StartedAt:      finishedAt.Add(-30 * time.Second),
		FinishedAt:     finishedAt,
		Error:          "pull failed",
	})

	output, err := executeRoot(t, []string{"history", "api"}, "")
	if err != nil {
		t.Fatalf("history command: %v", err)
	}

	for _, want := range []string{"JOB ID", "TYPE", "STATUS", "DATE", "ERROR", "job-1", "update", "failed", formatUnixTime(finishedAt.Unix()), "pull failed"} {
		if !strings.Contains(output, want) {
			t.Fatalf("history output %q does not contain %q", output, want)
		}
	}
}

func TestHistoryCommandReportsMissingDeploymentCleanly(t *testing.T) {
	setupTestHome(t)

	output, err := executeRoot(t, []string{"history", "missing-api"}, "")
	if err == nil {
		t.Fatal("history command succeeded with missing deployment")
	}
	if !strings.Contains(output, `deployment "missing-api" not found`) {
		t.Fatalf("history output %q does not contain clean not-found message", output)
	}
	for _, unwanted := range []string{"sql: no rows", "rpc error:", "code ="} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("history output %q contains %q", output, unwanted)
		}
	}
}

func TestJobCommandShowsDetailsAndLogs(t *testing.T) {
	setupTestHome(t)
	finishedAt := time.Unix(1_800_000_000, 0)
	insertJob(t, store.Job{
		ID:             "job-1",
		Type:           "deploy",
		DeploymentName: "api",
		Status:         store.JobStatusSucceeded,
		CreatedAt:      finishedAt.Add(-time.Minute),
		StartedAt:      finishedAt.Add(-30 * time.Second),
		FinishedAt:     finishedAt,
	})
	insertJobLog(t, "job-1", "Building api")
	insertJobLog(t, "job-1", "Deployment ready")

	output, err := executeRoot(t, []string{"job", "job-1", "--logs"}, "")
	if err != nil {
		t.Fatalf("job command: %v", err)
	}

	for _, want := range []string{"Job: job-1", "Type: deploy", "Deployment: api", "Status: succeeded", "Finished: " + formatUnixTime(finishedAt.Unix()), "Logs:", "Building api", "Deployment ready"} {
		if !strings.Contains(output, want) {
			t.Fatalf("job output %q does not contain %q", output, want)
		}
	}
}

func TestStatusCommandShowsMaskedEnvAndLatestUpdate(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	envPath := filepath.Join(location, ".env")
	if err := os.WriteFile(envPath, []byte("PORT=8080\nAUTH_URI=https://example.test\n"), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	insertRepository(t, store.Repository{
		Name:     "api_prod",
		URL:      "https://example.test/api.git",
		Location: location,
		EnvPath:  envPath,
	})
	finishedAt := time.Unix(1_800_000_000, 0)
	insertJob(t, store.Job{
		ID:             "job-1",
		Type:           "update",
		DeploymentName: "api_prod",
		Status:         store.JobStatusSucceeded,
		CreatedAt:      finishedAt.Add(-time.Minute),
		StartedAt:      finishedAt.Add(-30 * time.Second),
		FinishedAt:     finishedAt,
		Error:          "bad PORT=8080",
	})

	output, err := executeRoot(t, []string{"status", "api_prod"}, "")
	if err != nil {
		t.Fatalf("status command: %v", err)
	}

	for _, want := range []string{
		"Deployment: api_prod",
		"State: not_configured",
		"Latest update: update succeeded",
		"Containers:\n  none",
		"  AUTH_URI=*****",
		"  PORT=*****",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output %q does not contain %q", output, want)
		}
	}
	for _, leaked := range []string{"8080", "https://example.test\n"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("status output leaked env value %q: %q", leaked, output)
		}
	}
}

func TestStatusCommandReportsDockerUnavailableCleanly(t *testing.T) {
	setupTestHome(t)
	t.Setenv("DOCKER_HOST", fakeDockerHost())
	insertComposeRepository(t, "api")

	output, err := executeRoot(t, []string{"status", "api"}, "")
	if err == nil {
		t.Fatal("status command succeeded with unavailable Docker")
	}

	assertCleanDockerUnavailableOutput(t, output)
}

func TestStatusCommandReportsMissingDeploymentCleanly(t *testing.T) {
	setupTestHome(t)

	output, err := executeRoot(t, []string{"status", "missing-api"}, "")
	if err == nil {
		t.Fatal("status command succeeded with missing deployment")
	}

	if !strings.Contains(output, `deployment "missing-api" not found`) {
		t.Fatalf("status output %q does not contain clean not-found message", output)
	}
	for _, unwanted := range []string{"sql: no rows", "rpc error:", "code ="} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("status output %q contains %q", output, unwanted)
		}
	}
}

func TestPrintDeploymentLogsShowsEntriesAndHint(t *testing.T) {
	stream := &fakeDeploymentLogStream{entries: []*rpc.DeploymentLogEntry{
		{Container: "api-1", Message: "ready"},
		{Container: "worker-1", Message: "processed job"},
	}}

	var output bytes.Buffer
	if err := printDeploymentLogs(&output, stream, false); err != nil {
		t.Fatalf("print logs: %v", err)
	}

	for _, want := range []string{
		"api-1 | ready",
		"worker-1 | processed job",
		"Use --follow for live logs or --lines N for more history.",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("logs output %q does not contain %q", output.String(), want)
		}
	}
}

func TestLogsCommandRejectsNegativeLines(t *testing.T) {
	t.Cleanup(func() {
		if err := logsCmd.Flags().Set("lines", fmt.Sprint(defaultLogLines)); err != nil {
			t.Fatalf("reset logs lines flag: %v", err)
		}
	})

	output, err := executeRoot(t, []string{"logs", "api", "--lines", "-1"}, "")
	if err == nil {
		t.Fatal("logs command succeeded with negative lines")
	}
	if !strings.Contains(output, "lines must be greater than or equal to 0") {
		t.Fatalf("logs output %q does not contain validation error", output)
	}
	if strings.Contains(output, "Usage:") {
		t.Fatalf("logs output %q unexpectedly contains usage", output)
	}
}

func TestLogsCommandReportsDockerUnavailableCleanly(t *testing.T) {
	setupTestHome(t)
	t.Setenv("DOCKER_HOST", fakeDockerHost())
	insertComposeRepository(t, "api")

	output, err := executeRoot(t, []string{"logs", "api"}, "")
	if err == nil {
		t.Fatal("logs command succeeded with unavailable Docker")
	}

	assertCleanDockerUnavailableOutput(t, output)
}

func TestDockerBackedJobCommandsReportDockerUnavailableCleanly(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "build", args: []string{"build", "api"}},
		{name: "deploy", args: []string{"deploy", "api"}},
		{name: "stop", args: []string{"stop", "api"}},
		{name: "restart", args: []string{"restart", "api"}},
		{name: "update build", args: []string{"update", "api", "--build"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupTestHome(t)
			t.Setenv("DOCKER_HOST", fakeDockerHost())
			if tc.name == "update build" {
				sourceRepo := createGitRepository(t, map[string]string{
					"compose.yml": "services:\n  api:\n    build:\n      context: .\n",
					"Dockerfile":  "FROM scratch\n",
				})
				if output, err := executeRoot(t, []string{"create", sourceRepo, "--name", "api"}, ""); err != nil {
					t.Fatalf("create command: %v\n%s", err, output)
				}
			} else {
				insertComposeRepository(t, "api")
			}

			output, err := executeRoot(t, tc.args, "")
			if err == nil {
				t.Fatalf("%v succeeded with unavailable Docker", tc.args)
			}

			assertCleanDockerUnavailableOutput(t, output)
		})
	}
}

func TestRuntimeErrorsDoNotHideUsageErrors(t *testing.T) {
	output, err := executeRoot(t, []string{"status"}, "")
	if err == nil {
		t.Fatal("status command without args succeeded")
	}
	if !strings.Contains(output, "Usage:") {
		t.Fatalf("usage error output %q does not contain Usage", output)
	}
	if !strings.Contains(output, "deployctl status [repository-name]") {
		t.Fatalf("usage error output %q does not contain status usage", output)
	}
}

func TestDaemonStatusReportsDockerUnavailable(t *testing.T) {
	setupTestHome(t)
	t.Setenv("DOCKER_HOST", fakeDockerHost())

	output, err := executeRoot(t, []string{"daemon", "status"}, "")
	if err != nil {
		t.Fatalf("daemon status command: %v", err)
	}
	if !strings.Contains(output, "Docker") || !strings.Contains(output, "  Status: unavailable") {
		t.Fatalf("daemon status output %q does not report Docker unavailable", output)
	}
	if !strings.Contains(output, "Docker is unavailable") {
		t.Fatalf("daemon status output %q does not contain clean Docker error", output)
	}
}

func TestEnvCommandsSetListAndUnsetVariables(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: location})

	if _, err := executeRoot(t, []string{"env", "set", "api", "FOO=bar", "BAZ=qux"}, ""); err != nil {
		t.Fatalf("env set command: %v", err)
	}
	if _, err := executeRoot(t, []string{"env", "add", "api", "EXTRA=value"}, ""); err != nil {
		t.Fatalf("env add command: %v", err)
	}

	repository, err := getRepository(t, "api")
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
	if variables["FOO"] != "bar" || variables["BAZ"] != "qux" || variables["EXTRA"] != "value" {
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
	if _, ok := variables["FOO"]; ok || variables["BAZ"] != "qux" || variables["EXTRA"] != "value" {
		t.Fatalf("variables after unset = %#v", variables)
	}
}

func TestEnvImportCopiesEnvFile(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: location})

	envPath := filepath.Join(t.TempDir(), ".env")
	envContents := "# production\nFOO=bar\nBAZ='qux'\n"
	if err := os.WriteFile(envPath, []byte(envContents), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	if _, err := executeRoot(t, []string{"env", "import", "api", envPath}, ""); err != nil {
		t.Fatalf("env import file command: %v", err)
	}

	repository, err := getRepository(t, "api")
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

func TestEnvSetRejectsEnvFileWithoutImport(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: location})

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte("FOO=bar\n"), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	output, err := executeRoot(t, []string{"env", "set", "api", envPath}, "")
	if err == nil {
		t.Fatal("env set file command succeeded")
	}
	if !strings.Contains(output, "must use KEY=VALUE") {
		t.Fatalf("env set file output = %q", output)
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

	repository, err := getRepository(t, "api")
	if err != nil {
		t.Fatalf("get repository: %v", err)
	}
	if repository.EnvPath != "" {
		t.Fatalf("default env path = %q, want empty", repository.EnvPath)
	}
}

func TestEnvImportCopiesExplicitComposeEnvFile(t *testing.T) {
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

	if _, err := executeRoot(t, []string{"env", "import", "api", "backend.env", source}, ""); err != nil {
		t.Fatalf("env import explicit file copy command: %v", err)
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
	if !strings.Contains(output, "app.env\n") {
		t.Fatalf("env list output = %q", output)
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

func TestEnvListDiscoversMultipleEnvFiles(t *testing.T) {
	setupTestHome(t)
	location := t.TempDir()
	insertRepository(t, store.Repository{
		Name:        "api",
		URL:         "https://example.test/api.git",
		Location:    location,
		ComposePath: filepath.Join(location, "compose.yml"),
	})
	compose := `services:
  api:
    image: example/api
    env_file:
      - .env.api
      - env.missing
`
	if err := os.WriteFile(filepath.Join(location, "compose.yml"), []byte(compose), 0600); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(location, ".env.example"), []byte("EXAMPLE_ONLY=true\n"), 0600); err != nil {
		t.Fatalf("write example env file: %v", err)
	}

	if _, err := executeRoot(t, []string{"env", "set", "api", "PORT=8080"}, ""); err != nil {
		t.Fatalf("env set default command: %v", err)
	}
	if _, err := executeRoot(t, []string{"env", "set", "api", ".env.api", "API_KEY=token-value"}, ""); err != nil {
		t.Fatalf("env set api env command: %v", err)
	}
	if _, err := executeRoot(t, []string{"env", "set", "api", "env.secrets", "DATABASE_URL=postgres://example"}, ""); err != nil {
		t.Fatalf("env set secrets env command: %v", err)
	}

	output, err := executeRoot(t, []string{"env", "list", "api"}, "")
	if err != nil {
		t.Fatalf("env list command: %v", err)
	}
	for _, want := range []string{".env\n", "PORT=*****", ".env.api\n", "API_KEY=*****", "env.secrets\n", "DATABASE_URL=*****"} {
		if !strings.Contains(output, want) {
			t.Fatalf("env list output %q does not contain %q", output, want)
		}
	}
	for _, want := range []string{"Warning: compose references missing env files:", "env.missing"} {
		if !strings.Contains(output, want) {
			t.Fatalf("env list output %q does not contain %q", output, want)
		}
	}
	for _, leaked := range []string{"8080", "token-value", "postgres://example"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("env list leaked value %q in output %q", leaked, output)
		}
	}
	if strings.Contains(output, ".env.example") || strings.Contains(output, "EXAMPLE_ONLY") {
		t.Fatalf("env list output included example env file: %q", output)
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
	if _, err := getRepository(t, "api"); !store.IsNotFound(err) {
		t.Fatalf("repository lookup after delete error = %v, want not found", err)
	}
}

func TestDeploymentCommandsReportMissingComposeFile(t *testing.T) {
	setupTestHome(t)
	insertRepository(t, store.Repository{Name: "api", URL: "https://example.test/api.git", Location: t.TempDir()})

	for _, args := range [][]string{
		{"build", "api"},
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
			got, err := confirmDelete(strings.NewReader(tc.input), io.Discard, "api", false)
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

	_, err := executeRootCommand(&output)
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
	server, err := service.NewServer()
	if err != nil {
		t.Fatalf("new test server: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Close()
	})
	grpcServer := service.NewGRPCServer(server)
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

	dataStore := openTestStore(t)
	defer dataStore.Close()
	if err := dataStore.Repositories.Insert(context.Background(), repository); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()

	dataStore, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return dataStore
}

func getRepository(t *testing.T, name string) (store.Repository, error) {
	t.Helper()

	dataStore := openTestStore(t)
	defer dataStore.Close()
	return dataStore.Repositories.Get(context.Background(), name)
}

func insertJob(t *testing.T, job store.Job) {
	t.Helper()

	dataStore := openTestStore(t)
	defer dataStore.Close()
	if err := dataStore.Jobs.Insert(context.Background(), job); err != nil {
		t.Fatalf("insert job: %v", err)
	}
}

func insertJobLog(t *testing.T, jobID string, message string) {
	t.Helper()

	dataStore := openTestStore(t)
	defer dataStore.Close()
	if _, err := dataStore.Jobs.AddLog(context.Background(), jobID, message); err != nil {
		t.Fatalf("insert job log: %v", err)
	}
}

func insertComposeRepository(t *testing.T, name string) {
	t.Helper()

	location := t.TempDir()
	composePath := filepath.Join(location, "compose.yml")
	if err := os.WriteFile(composePath, []byte("services:\n  api:\n    build:\n      context: .\n"), 0644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(location, "Dockerfile"), []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	insertRepository(t, store.Repository{
		Name:        name,
		URL:         "https://example.test/api.git",
		Location:    location,
		ComposePath: composePath,
	})
}

func assertCleanDockerUnavailableOutput(t *testing.T, output string) {
	t.Helper()

	if count := strings.Count(output, "Docker is unavailable"); count != 1 {
		t.Fatalf("Docker unavailable output count = %d, output = %q", count, output)
	}
	for _, unwanted := range []string{"Usage:", "rpc error:", "code = Unknown"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("Docker unavailable output %q contains %q", output, unwanted)
		}
	}
}

func fakeDockerHost() string {
	return "unix://" + filepath.Join(os.TempDir(), fmt.Sprintf("dct-missing-%d-%d.sock", os.Getpid(), time.Now().UnixNano()))
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

type fakeDeploymentLogStream struct {
	entries []*rpc.DeploymentLogEntry
}

func (s *fakeDeploymentLogStream) Recv() (*rpc.DeploymentLogEntry, error) {
	if len(s.entries) == 0 {
		return nil, io.EOF
	}
	entry := s.entries[0]
	s.entries = s.entries[1:]
	return entry, nil
}
