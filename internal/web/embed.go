package web

import "embed"

//go:embed all:public
var frontendFS embed.FS
