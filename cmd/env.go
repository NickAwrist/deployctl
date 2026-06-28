package cmd

import (
	"errors"
	"fmt"
	"strings"

	"deployctl/internal/envfile"
	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage deployment env variables",
}

var envSetCmd = &cobra.Command{
	Use:               "set [repository-name] [env-file] KEY=VALUE...|ENV_FILE",
	Aliases:           []string{"add"},
	Short:             "Set deployment env variables",
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		targetEnvFile, values := resolveEnvSetArgs(args[1:])
		if len(values) == 1 && !strings.Contains(values[0], "=") {
			return runWithClient(cmd, func(client *daemonClient) error {
				response, err := client.Env.ImportEnvFile(cmd.Context(), &rpc.ImportEnvFileRequest{
					DeploymentName: repositoryName,
					SourcePath:     values[0],
					EnvFile:        targetEnvFile,
				})
				if err != nil {
					return err
				}
				return handleJob(cmd, client, response, fmt.Sprintf("Updated env file for %s", repositoryName))
			})
		}

		variables, err := parseAssignments(values)
		if err != nil {
			return err
		}
		for name := range variables {
			if err := envfile.ValidateName(name); err != nil {
				return err
			}
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Env.SetEnv(cmd.Context(), &rpc.SetEnvRequest{
				DeploymentName: repositoryName,
				Variables:      variables,
				EnvFile:        targetEnvFile,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, fmt.Sprintf("Updated %d env variable(s) for %s", len(values), repositoryName))
		})
	},
}

var envUnsetCmd = &cobra.Command{
	Use:               "unset [repository-name] [env-file] KEY...",
	Aliases:           []string{"delete", "remove", "rm"},
	Short:             "Delete deployment env variables",
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		targetEnvFile, names := resolveEnvUnsetArgs(args[1:])
		for _, name := range names {
			if err := envfile.ValidateName(name); err != nil {
				return err
			}
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Env.UnsetEnv(cmd.Context(), &rpc.UnsetEnvRequest{
				DeploymentName: repositoryName,
				Names:          names,
				EnvFile:        targetEnvFile,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, fmt.Sprintf("Deleted env variable(s) from %s", repositoryName))
		})
	},
}

var envListCmd = &cobra.Command{
	Use:               "list [repository-name] [env-file]",
	Short:             "List deployment env variables",
	Args:              cobra.RangeArgs(1, 2),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		envFile := ""
		if len(args) == 2 {
			envFile = args[1]
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Env.ListEnvNames(cmd.Context(), &rpc.ListEnvNamesRequest{
				DeploymentName: repositoryName,
				EnvFile:        envFile,
			})
			if err != nil {
				return err
			}
			if len(response.Names) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No env variables found for %s\n", repositoryName)
				return nil
			}
			printMaskedEnv(cmd.OutOrStdout(), response.Names)
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envSetCmd, envUnsetCmd, envListCmd)
	addJobFlags(envSetCmd)
	addJobFlags(envUnsetCmd)
}

func resolveEnvSetArgs(values []string) (string, []string) {
	if len(values) >= 2 && !strings.Contains(values[0], "=") {
		return values[0], values[1:]
	}
	return "", values
}

func resolveEnvUnsetArgs(names []string) (string, []string) {
	if len(names) >= 2 && !looksLikeEnvName(names[0]) {
		return names[0], names[1:]
	}
	return "", names
}

func looksLikeEnvName(value string) bool {
	return envfile.ValidateName(value) == nil
}
