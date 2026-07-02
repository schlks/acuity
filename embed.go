package acuity

import "embed"

//go:embed web/templates web/static
var WebFS embed.FS
