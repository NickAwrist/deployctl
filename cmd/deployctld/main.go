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

	logger := service.NewDaemonLogger()
	listener, err := service.ListenUnix(socketPath)
	if err != nil {
		logger.Printf("listen on %s failed: %v", socketPath, err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	logger.Printf("deployctld listening on %s", socketPath)
	if err := service.NewServerWithLogger(logger).Serve(listener); err != nil {
		logger.Printf("serve failed: %v", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
