package main

import (
	"os"

	"mica/cli/internal/cliapp"
)

func main() {
	os.Exit(cliapp.Run(os.Args[1:]))
}
