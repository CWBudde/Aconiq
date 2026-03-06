package main

import (
	"os"

	"github.com/soundplan/soundplan/backend/internal/app/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
