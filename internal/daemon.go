package internal

import (
	"os"
	"path/filepath"
)

const SocketFileName = "deployctld.sock"

func GetSocketPath() string {
	if path := os.Getenv("DEPLOYCTL_SOCKET_PATH"); path != "" {
		return path
	}
	return filepath.Join(GetMainDirectory(), SocketFileName)
}
