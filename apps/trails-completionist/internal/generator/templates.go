package generator

import "embed"

//go:embed *.tmpl *.js *.css
var Templates embed.FS
