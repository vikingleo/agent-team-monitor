package web

import "embed"

//go:embed all:static
var StaticFiles embed.FS
