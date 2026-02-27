package main

import (
	"fmt"
	"os"

	"github.com/alexandreferreira/gitm/internal/cli"
)

func main() {
	if err := cli.Root().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
