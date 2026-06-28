package git

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"deployctl/internal"
)

func CloneRepo(ctx context.Context, repoURL string, name string, log func(string)) (string, error) {
	if name == "" {
		name = repoNameFromURL(repoURL)
	}
	repoPath := filepath.Join(internal.GetRepositoryDirectory(), name)

	if err := runGitCommand(ctx, "", log, "clone", repoURL, repoPath); err != nil {
		return "", cloneError(repoURL, err)
	}

	if log != nil {
		log(fmt.Sprintf("Repository cloned into %s", repoPath))
	}
	return repoPath, nil
}

func isSSHURL(repoURL string) bool {
	repoURL = strings.TrimSpace(repoURL)
	if strings.HasPrefix(repoURL, "ssh://") {
		return true
	}

	return strings.Contains(repoURL, "@") && strings.Contains(repoURL, ":") && !strings.Contains(repoURL, "://")
}

func cloneError(repoURL string, err error) error {
	if isSSHURL(repoURL) {
		return fmt.Errorf("%w\n\nFor private SSH repositories, deployctl runs git clone directly, so your local git and SSH configuration is used. Check that ssh -T git@github.com works for this repository host.", err)
	}

	return fmt.Errorf("%w\n\nFor private HTTPS repositories, deployctl runs git clone directly, so your local git credential helper is used. For GitHub, authenticate git with GitHub CLI, Git Credential Manager, or a personal access token.", err)
}

func repoNameFromURL(repoURL string) string {
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimSuffix(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	if i := strings.LastIndex(repoURL, ":"); i >= 0 {
		repoURL = repoURL[i+1:]
	}

	return filepath.Base(repoURL)
}

func runGitCommand(ctx context.Context, directory string, log func(string), args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if directory != "" {
		cmd.Dir = directory
	}

	output, err := cmd.CombinedOutput()
	for _, line := range outputLines(string(output)) {
		if log != nil {
			log(line)
		}
	}
	if err != nil {
		return fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}

func outputLines(output string) []string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
