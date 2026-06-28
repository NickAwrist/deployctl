package cmd

import (
	"fmt"
	"os"

	"deployctl/internal"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "deployctl",
	Short:   "A deployment control CLI",
	Version: internal.GitCommit(),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), "deployctl")
	},
}

func init() {
	rootCmd.SetVersionTemplate("deployctl build: git commit {{.Version}}\n")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
