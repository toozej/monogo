//go:build js && wasm

// Package main builds the gocicle browser client.
package main

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"github.com/toozej/monogo/apps/gocicle/internal/ui"
)

func main() { ui.RegisterRoutes(); app.RunWhenOnBrowser() }
