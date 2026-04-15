package gui

import "embed"

//go:embed appicon.png
var appIcon []byte

//go:embed all:frontend/dist
var assets embed.FS
