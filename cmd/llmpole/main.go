package main

import (
	"fmt"
	"os"

	"github.com/shayne-snap/llmpole/internal/cli"
)

// Version is set at build time via -ldflags "-X main.Version=...". Default "dev" for local builds.
var Version = "dev"

func main() {
	cli.Version = Version
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
