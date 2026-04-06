package web

import "embed"

// Files contains the embedded static frontend assets.
//
//go:embed index.html
var Files embed.FS
