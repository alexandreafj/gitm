package main

import (
	"fmt"
	"os"

	"github.com/alexandreferreira/gitm/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Root(version).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
