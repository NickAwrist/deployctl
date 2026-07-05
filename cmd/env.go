package cmd

import (
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
	Use:               "set [deployment-name] [env-file] KEY=VALUE...",
	Aliases:           []string{"add"},
	Short:             "Set deployment env variables",
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		deploymentName, err := deploymentNameArg(args)
		if err != nil {
			return err
		}

		targetEnvFile, values := resolveEnvSetArgs(args[1:])
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
				DeploymentName: deploymentName,
				Variables:      variables,
				EnvFile:        targetEnvFile,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, envChangeSuccess("Updated", len(variables), "for", deploymentName))
		})
	},
}

var envImportCmd = &cobra.Command{
	Use:               "import [deployment-name] [env-file] env-path",
	Short:             "Import a deployment env file",
	Args:              cobra.RangeArgs(2, 3),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		deploymentName, err := deploymentNameArg(args)
		if err != nil {
			return err
		}

		targetEnvFile, sourcePath := resolveEnvImportArgs(args[1:])
		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Env.ImportEnvFile(cmd.Context(), &rpc.ImportEnvFileRequest{
				DeploymentName: deploymentName,
				SourcePath:     sourcePath,
				EnvFile:        targetEnvFile,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, envFileImportedSuccess(deploymentName))
		})
	},
}

var envUnsetCmd = &cobra.Command{
	Use:               "unset [deployment-name] [env-file] KEY...",
	Aliases:           []string{"delete", "remove", "rm"},
	Short:             "Delete deployment env variables",
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		deploymentName, err := deploymentNameArg(args)
		if err != nil {
			return err
		}

		targetEnvFile, names := resolveEnvUnsetArgs(args[1:])
		for _, name := range names {
			if err := envfile.ValidateName(name); err != nil {
				return err
			}
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Env.UnsetEnv(cmd.Context(), &rpc.UnsetEnvRequest{
				DeploymentName: deploymentName,
				Names:          names,
				EnvFile:        targetEnvFile,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, envChangeSuccess("Deleted", len(names), "from", deploymentName))
		})
	},
}

var envListCmd = &cobra.Command{
	Use:               "list [deployment-name] [env-file]",
	Short:             "List deployment env variables",
	Args:              cobra.RangeArgs(1, 2),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		deploymentName, err := deploymentNameArg(args)
		if err != nil {
			return err
		}

		envFile := ""
		if len(args) == 2 {
			envFile = args[1]
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Env.ListEnvFiles(cmd.Context(), &rpc.ListEnvFilesRequest{
				DeploymentName: deploymentName,
				EnvFile:        envFile,
			})
			if err != nil {
				return err
			}
			printEnvFiles(cmd.OutOrStdout(), deploymentName, envFile != "", response)
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envSetCmd, envImportCmd, envUnsetCmd, envListCmd)
	addJobFlags(envSetCmd)
	addJobFlags(envImportCmd)
	addJobFlags(envUnsetCmd)
}

func resolveEnvSetArgs(values []string) (string, []string) {
	if len(values) >= 2 && !strings.Contains(values[0], "=") {
		return values[0], values[1:]
	}
	return "", values
}

func resolveEnvImportArgs(values []string) (string, string) {
	if len(values) == 1 {
		return "", values[0]
	}
	return values[0], values[1]
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
