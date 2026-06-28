package internal

import (
	"fmt"
	"os"

	"path/filepath"
)

func CheckError(err error) {
	if err == nil {
		return
	}

	fmt.Printf("\x1b[31;1m%s\x1b[0m\n", fmt.Sprintf("error: %s", err))
	os.Exit(1)
}

func GetMainDirectory() string {
	home, err := os.UserHomeDir()
	CheckError(err)
	return filepath.Join(home, ".deployctl")
}

func GetRepositoryDirectory() string {
	return filepath.Join(GetMainDirectory(), "repositories")
}

func InitializeDirectoryStructure() {
	err := os.MkdirAll(GetMainDirectory(), 0755)
	CheckError(err)
	err = os.MkdirAll(GetRepositoryDirectory(), 0755)
	CheckError(err)
}
