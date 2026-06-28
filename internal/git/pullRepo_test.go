package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPullRepoFastForwardsCheckout(t *testing.T) {
	source := t.TempDir()
	writeFile(t, filepath.Join(source, "app.txt"), "v1\n")
	runGit(t, source, "init")
	runGit(t, source, "add", ".")
	runGit(t, source, "-c", "user.name=deployctl", "-c", "user.email=deployctl@example.test", "commit", "-m", "initial")

	checkout := filepath.Join(t.TempDir(), "checkout")
	runGit(t, "", "clone", source, checkout)

	writeFile(t, filepath.Join(source, "app.txt"), "v2\n")
	runGit(t, source, "add", ".")
	runGit(t, source, "-c", "user.name=deployctl", "-c", "user.email=deployctl@example.test", "commit", "-m", "update")

	if err := PullRepo(context.Background(), checkout, nil); err != nil {
		t.Fatalf("pull repo: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(checkout, "app.txt"))
	if err != nil {
		t.Fatalf("read checkout file: %v", err)
	}
	if strings.ReplaceAll(string(got), "\r\n", "\n") != "v2\n" {
		t.Fatalf("checkout file = %q, want v2", got)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()

	command := exec.Command("git", args...)
	if directory != "" {
		command.Dir = directory
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
