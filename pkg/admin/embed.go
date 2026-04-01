package admin

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed static
var distFS embed.FS

var hasDistFS bool

func init() {
	hasDistFS = checkStaticFiles()
}

func checkStaticFiles() bool {
	_, err := fs.Stat(distFS, "static")
	if err != nil {
		return false
	}
	f, err := fs.Stat(distFS, "static/index.html")
	return err == nil && f != nil
}

func GetDistFS() fs.FS {
	if hasDistFS {
		sub, _ := fs.Sub(distFS, "static")
		return sub
	}
	return nil
}

func HasEmbeddedUI() bool {
	return hasDistFS
}

func GetStaticDir() string {
	if hasDistFS {
		return "static"
	}
	return ""
}

func StaticDirExists() bool {
	if _, err := os.Stat("pkg/admin/static/index.html"); err == nil {
		return true
	}
	return false
}
