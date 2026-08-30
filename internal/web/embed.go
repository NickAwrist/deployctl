package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the sub-filesystem containing compiled web assets.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
