package git

import (
	"path/filepath"
	"strings"

	"deployctl/internal"

	"github.com/go-git/go-git/v5"
)

func CloneRepo(repoURL string, name string) error {
	if name == "" {
		name = repoNameFromURL(repoURL)
	}
	repoPath := filepath.Join(internal.GetRepositoryDirectory(), name)
	_, err := git.PlainClone(repoPath, false, &git.CloneOptions{
		URL: repoURL,
	})
	if err != nil {
		return err
	}
	internal.Info("Repository cloned successfully into %s", repoPath)
	return nil
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
