package git

import (
	"context"
	"fmt"
)

func PullRepo(ctx context.Context, repoPath string, log func(string)) error {
	if err := runGitCommand(ctx, repoPath, log, "pull", "--ff-only"); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}
	return nil
}
