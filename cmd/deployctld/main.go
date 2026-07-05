package main

import (
	"fmt"
	"os"

	"deployctl/internal"
	"deployctl/internal/service"
)

func main() {
	if err := internal.InitializeDirectoryStructure(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	socketPath, err := internal.SocketPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(os.Args) > 1 {
		socketPath = os.Args[1]
	}

	logger, err := service.NewDaemonLogger()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	listener, err := service.ListenUnix(socketPath)
	if err != nil {
		logger.Printf("listen on %s failed: %v", socketPath, err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	logger.Printf("deployctld listening on %s", socketPath)
	server, err := service.NewServerWithLogger(logger)
	if err != nil {
		logger.Printf("open server failed: %v", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer server.Close()
	if err := server.Serve(listener); err != nil {
		logger.Printf("serve failed: %v", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
