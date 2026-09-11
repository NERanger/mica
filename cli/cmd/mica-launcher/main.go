package main

import (
	"fmt"
	"os"

	"mica/cli/internal/launcher"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: mica-launcher deployment.toml")
		os.Exit(2)
	}
	os.Exit(launcher.Run(os.Args[1]))
}
