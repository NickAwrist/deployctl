package main

import (
	"flag"
	"net"
	"os"

	"deployctl/internal"
	"deployctl/internal/service"
	"deployctl/internal/web"
)

func main() {
	if err := internal.InitializeDirectoryStructure(); err != nil {
		exitWithError(err)
	}

	var socketPath string
	var httpAddr string
	flag.StringVar(&socketPath, "socket", "", "Unix socket path for gRPC server")
	flag.StringVar(&httpAddr, "http-addr", "", "HTTP address for web dashboard (e.g. 127.0.0.1:7123)")
	flag.Parse()

	if socketPath == "" {
		if len(flag.Args()) > 0 {
			socketPath = flag.Args()[0]
		} else {
			var err error
			socketPath, err = internal.SocketPath()
			if err != nil {
				exitWithError(err)
			}
		}
	}

	if httpAddr == "" {
		httpAddr = internal.HTTPAddr()
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
	defer listener.Close()

	logger.Printf("deployctld listening on %s", socketPath)
	server, err := service.NewServerWithLogger(logger)
	if err != nil {
		logger.Printf("open server failed: %v", err)
		exitWithError(err)
	}
	defer server.Close()

	if httpAddr != "" && httpAddr != "none" && httpAddr != "disabled" {
		httpListener, err := net.Listen("tcp", httpAddr)
		if err != nil {
			logger.Printf("listen for web UI on %s failed: %v", httpAddr, err)
		} else {
			defer httpListener.Close()
			logger.Printf("deployctl web UI listening on http://%s", httpAddr)
			webServer := web.NewServer(server)
			go func() {
				if err := webServer.Serve(httpListener); err != nil {
					logger.Printf("web server stopped: %v", err)
				}
			}()
		}
	}

	if err := server.Serve(listener); err != nil {
		logger.Printf("serve failed: %v", err)
		exitWithError(err)
	}
}

func exitWithError(err error) {
	_, _ = os.Stderr.WriteString(err.Error() + "\n")
	os.Exit(1)
}
