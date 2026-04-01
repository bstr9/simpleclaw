package admin

import (
	"embed"
	"io/fs"
	"os"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

//go:embed all:static
var distFS embed.FS

var hasDistFS bool

func init() {
	hasDistFS = checkStaticFiles()
	logger.Info("[Admin Embed] Checking static files", zap.Bool("hasDistFS", hasDistFS))
}

func checkStaticFiles() bool {
	entries, err := fs.ReadDir(distFS, "static")
	if err != nil {
		logger.Warn("[Admin Embed] Cannot read static dir", zap.Error(err))
		return false
	}
	logger.Info("[Admin Embed] Static dir entries", zap.Int("count", len(entries)))
	for _, e := range entries[:min(5, len(entries))] {
		logger.Info("[Admin Embed] Entry", zap.String("name", e.Name()), zap.Bool("isDir", e.IsDir()))
	}

	f, err := fs.Stat(distFS, "static/index.html")
	if err != nil {
		logger.Warn("[Admin Embed] Cannot find index.html", zap.Error(err))
		return false
	}
	logger.Info("[Admin Embed] Found index.html", zap.Int64("size", f.Size()))

	assetsDir, err := fs.ReadDir(distFS, "static/assets")
	if err != nil {
		logger.Warn("[Admin Embed] Cannot read assets dir", zap.Error(err))
		return false
	}
	logger.Info("[Admin Embed] Assets count", zap.Int("count", len(assetsDir)))

	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
