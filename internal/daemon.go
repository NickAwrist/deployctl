package internal

import (
	"os"
	"path/filepath"
)

const SocketFileName = "deployctld.sock"
const DaemonLogFileName = "deployctld.log"

func GetSocketPath() string {
	if path := os.Getenv("DEPLOYCTL_SOCKET_PATH"); path != "" {
		return path
	}
	return filepath.Join(GetMainDirectory(), SocketFileName)
}

func GetDaemonLogPath() string {
	if path := os.Getenv("DEPLOYCTL_LOG_PATH"); path != "" {
		return path
	}
	return filepath.Join(GetMainDirectory(), DaemonLogFileName)
}
