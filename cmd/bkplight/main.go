package main

import (
	"context"
	"os"

	"bkplight.local/bkplight/internal/cli"
)

func main() {
	// Serve owns signal registration and orderly resource release; read-only CLI
	// commands use the same safe boundary without acquiring daemon state.
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
