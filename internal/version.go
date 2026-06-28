package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

const unknownGitCommit = "unknown"

// GitCommit returns the short VCS revision embedded by the Go toolchain.
func GitCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return gitCommitFromSourceCheckout()
	}

	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return shortGitCommit(setting.Value)
		}
	}

	return gitCommitFromSourceCheckout()
}

func shortGitCommit(revision string) string {
	if revision == "" {
		return unknownGitCommit
	}
	if len(revision) < 7 {
		return revision
	}
	return revision[:7]
}

func gitCommitFromSourceCheckout() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return unknownGitCommit
	}

	repositoryRoot := filepath.Dir(filepath.Dir(file))
	if _, err := os.Stat(filepath.Join(repositoryRoot, ".git")); err != nil {
		return unknownGitCommit
	}

	output, err := exec.Command("git", "-C", repositoryRoot, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return unknownGitCommit
	}

	return shortGitCommit(strings.TrimSpace(string(output)))
}
