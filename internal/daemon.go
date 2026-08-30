package internal

import (
	"os"
	"path/filepath"
)

const SocketFileName = "deployctld.sock"
const DaemonLogFileName = "deployctld.log"
const DefaultHTTPAddr = "127.0.0.1:7123"

func SocketPath() (string, error) {
	if path := os.Getenv("DEPLOYCTL_SOCKET_PATH"); path != "" {
		return path, nil
	}
	mainDirectory, err := MainDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(mainDirectory, SocketFileName), nil
}

func DaemonLogPath() (string, error) {
	if path := os.Getenv("DEPLOYCTL_LOG_PATH"); path != "" {
		return path, nil
	}
	mainDirectory, err := MainDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(mainDirectory, DaemonLogFileName), nil
}

func HTTPAddr() string {
	if addr := os.Getenv("DEPLOYCTL_HTTP_ADDR"); addr != "" {
		return addr
	}
	return DefaultHTTPAddr
}
