//go:build !js || !wasm

// Package main — non-WASM stub so `go build ./...` succeeds on native targets.
package main

func main() {}
