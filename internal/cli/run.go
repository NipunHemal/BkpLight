// Package cli implements the command boundary and its safe output contract.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"bkplight.local/bkplight/internal/safeerr"
)

func Run(_ context.Context, args []string, out, errOut io.Writer) int {
	invalidCommand := func() int {
		fmt.Fprintln(errOut, (&safeerr.Error{Code: safeerr.Invalid, Field: "command"}).Error())
		return 2
	}
	if len(args) == 0 {
		return invalidCommand()
	}
	switch args[0] {
	case "version":
		flags := flag.NewFlagSet("version", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
			return invalidCommand()
		}
		fmt.Fprintln(out, "bkplight dev")
		return 0
	default:
		return invalidCommand()
	}
}
