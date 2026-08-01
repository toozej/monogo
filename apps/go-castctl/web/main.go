//go:build js && wasm

// Package main builds the go-app WebAssembly client.
package main

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"github.com/toozej/monogo/apps/go-castctl/internal/ui"
)

func main() {
	ui.RegisterRoutes()
	app.RunWhenOnBrowser()
}
