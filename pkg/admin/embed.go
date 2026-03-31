package admin

import (
	"embed"
	"io/fs"
)

var (
	distFS    embed.FS
	hasDistFS bool
)

func GetDistFS() fs.FS {
	if hasDistFS {
		return distFS
	}
	return nil
}

func HasEmbeddedUI() bool {
	return hasDistFS
}
