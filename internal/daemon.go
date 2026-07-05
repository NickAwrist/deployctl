package internal

import (
	"os"
	"path/filepath"
)

const SocketFileName = "deployctld.sock"
const DaemonLogFileName = "deployctld.log"

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
