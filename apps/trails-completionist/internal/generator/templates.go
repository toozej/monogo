package generator

import "embed"

//go:embed *.tmpl app.js styles.css
var Templates embed.FS
