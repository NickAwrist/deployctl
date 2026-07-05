package main

import (
	"os"

	"deployctl/internal"
	"deployctl/internal/service"
)

func main() {
	if err := internal.InitializeDirectoryStructure(); err != nil {
		exitWithError(err)
	}

	socketPath, err := internal.SocketPath()
	if err != nil {
		exitWithError(err)
	}
	if len(os.Args) > 1 {
		socketPath = os.Args[1]
	}

	logger, err := service.NewDaemonLogger()
	if err != nil {
		exitWithError(err)
	}
	listener, err := service.ListenUnix(socketPath)
	if err != nil {
		logger.Printf("listen on %s failed: %v", socketPath, err)
		exitWithError(err)
	}
	logger.Printf("deployctld listening on %s", socketPath)
	server, err := service.NewServerWithLogger(logger)
	if err != nil {
		logger.Printf("open server failed: %v", err)
		exitWithError(err)
	}
	defer server.Close()
	if err := server.Serve(listener); err != nil {
		logger.Printf("serve failed: %v", err)
		exitWithError(err)
	}
}

func exitWithError(err error) {
	_, _ = os.Stderr.WriteString(err.Error() + "\n")
	os.Exit(1)
}
