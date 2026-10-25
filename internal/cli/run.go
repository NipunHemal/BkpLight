// Package cli implements the command boundary and its safe output contract.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"bkplight.local/bkplight/internal/config"
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
	case "config":
		if len(args) < 2 || args[1] != "validate" {
			return invalidCommand()
		}
		flags := flag.NewFlagSet("config validate", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		path := flags.String("config", "", "configuration file")
		if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 || *path == "" {
			return invalidCommand()
		}
		file, err := os.Open(*path)
		if err != nil {
			fmt.Fprintln(errOut, (&safeerr.Error{Code: safeerr.Invalid, Field: "config"}).Error())
			return 2
		}
		defer file.Close()
		c, err := config.Decode(file)
		if err != nil {
			public := safeerr.Public(err)
			fmt.Fprintln(errOut, public.Error())
			return 2
		}
		revision, _, err := config.Revision(c)
		if err != nil {
			public := safeerr.Public(err)
			fmt.Fprintln(errOut, public.Error())
			return 2
		}
		fmt.Fprintln(out, "valid", revision)
		return 0
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
