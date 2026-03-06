package main

import (
	"os"

	"github.com/aconiq/backend/internal/app/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
