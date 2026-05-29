package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"deployctl/internal"
)

func CloneRepo(repoURL string, name string) (string, error) {
	if name == "" {
		name = repoNameFromURL(repoURL)
	}
	repoPath := filepath.Join(internal.GetRepositoryDirectory(), name)

	if err := cloneWithGit(repoURL, repoPath); err != nil {
		return "", cloneError(repoURL, err)
	}

	internal.Info("Repository cloned successfully into %s", repoPath)
	return repoPath, nil
}

func cloneWithGit(repoURL string, repoPath string) error {
	cmd := exec.Command("git", "clone", repoURL, repoPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	return nil
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
