package main

import (
	"fmt"
	"os"

	"deployctl/internal"
	"deployctl/internal/service"
)

func main() {
	internal.InitializeDirectoryStructure()

	socketPath := internal.GetSocketPath()
	if len(os.Args) > 1 {
		socketPath = os.Args[1]
	}

	listener, err := service.ListenUnix(socketPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("deployctld listening on %s\n", socketPath)
	if err := service.NewServer().Serve(listener); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
