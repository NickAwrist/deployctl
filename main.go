package main

import (
	"fmt"
	"os"

	"deployctl/cmd"
	"deployctl/internal"
)

func main() {
	if err := internal.InitializeDirectoryStructure(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cmd.Execute()
}
