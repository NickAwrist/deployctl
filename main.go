package main

import (
	"deployctl/cmd"
	"deployctl/internal"
)

func main() {
	internal.InitializeDirectoryStructure()
	cmd.Execute()
}
