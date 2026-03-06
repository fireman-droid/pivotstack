package main

import (
	"fmt"
	"os"

	"kiro-api-proxy/cmd/kiro-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
