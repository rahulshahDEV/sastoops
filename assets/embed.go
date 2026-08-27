package assets

import "embed"

//go:embed apps recipes modules
var FS embed.FS
