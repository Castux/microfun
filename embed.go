package main

import "embed"

// The embedded standard library, shared by the native (main.go) and browser
// (main_wasm.go) entry points.
//
//go:embed core
var coreFS embed.FS
