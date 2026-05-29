package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"deployctl/internal"
	internalfile "deployctl/internal/file"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

/*
deployctl delete <repository-name>

Deletes a deployment.

Arguments:

	<repository-name> The name of the deployment to delete
*/
var deleteCmd = &cobra.Command{
	Use:               "delete [repository-name]",
	Short:             "Delete a deployment",
	Aliases:           []string{"remove", "rm"},
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the repository name from the command line arguments or flags
		repositoryName := ""
		if len(args) > 0 {
			repositoryName = args[0]
			if repositoryName == "" {
				return errors.New("repository name is required")
			}
		}

		// Get the force flag from the command line arguments or flags
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}

		// Get the repository from the database
		repositories := store.NewRepositoryStore()
		repository, err := repositories.Get(cmd.Context(), repositoryName)
		if err != nil {
			return err
		}

		// Prompt the user for confirmation
		confirmed, err := confirmDelete(cmd.InOrStdin(), repository.Name, force)
		if err != nil {
			return err
		}
		if !confirmed && !force {
			internal.Warning("Delete cancelled")
			return nil
		}

		// Delete the repository from the database and the file system
		if err := internalfile.RemoveAllInside(internal.GetRepositoryDirectory(), repository.Location); err != nil {
			return err
		}
		if err := repositories.Delete(cmd.Context(), repository.Name); err != nil {
			return err
		}

		internal.Info("Deleted deployment %s", repository.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().BoolP("force", "f", false, "Force deletion without confirmation")
}

func confirmDelete(input io.Reader, repositoryName string, force bool) (bool, error) {
	if force {
		return true, nil
	}

	fmt.Fprintf(os.Stdout, "Are you sure you want to permanently delete %s? (Y/n) ", repositoryName)

	reader := bufio.NewReader(input)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	answer = strings.TrimSpace(answer)
	return answer == "Y", nil
}
