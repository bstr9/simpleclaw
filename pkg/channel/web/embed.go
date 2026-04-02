package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticFS embed.FS

func GetStaticFS() fs.FS {
	sub, _ := fs.Sub(staticFS, "static")
	return sub
}

func HasEmbeddedUI() bool {
	_, err := fs.Stat(staticFS, "static/index.html")
	return err == nil
}
