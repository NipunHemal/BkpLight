// Package cli implements the command boundary and its safe output contract.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"bkplight.local/bkplight/internal/app"
	"bkplight.local/bkplight/internal/config"
	"bkplight.local/bkplight/internal/control"
	"bkplight.local/bkplight/internal/safeerr"
	"bkplight.local/bkplight/internal/secrets"
)

func Run(ctx context.Context, args []string, out, errOut io.Writer) int {
	invalidCommand := func() int {
		fmt.Fprintln(errOut, (&safeerr.Error{Code: safeerr.Invalid, Field: "command"}).Error())
		return 2
	}
	if len(args) == 0 {
		return invalidCommand()
	}
	switch args[0] {
	case "serve":
		flags := flag.NewFlagSet("serve", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		path := flags.String("config", "", "configuration file")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *path == "" {
			return invalidCommand()
		}
		if err := app.Serve(ctx, *path); err != nil {
			public := safeerr.Public(err)
			fmt.Fprintln(errOut, public.Error())
			return 3
		}
		return 0
	case "health", "jobs", "history", "operation":
		return runControl(ctx, args, out, errOut)
	case "backup", "upload", "verify", "restore", "retention", "notification", "notifications", "source", "destination", "deliveries", "catalog":
		fmt.Fprintln(errOut, (&safeerr.Error{Code: safeerr.NotImplemented}).Error())
		return 4
	case "config":
		if len(args) < 2 || args[1] != "validate" {
			return invalidCommand()
		}
		flags := flag.NewFlagSet("config validate", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		path := flags.String("config", "", "configuration file")
		checkSecrets := flags.Bool("check-secrets", false, "check active secret references")
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
		if refs := config.SecretRefs(c); *checkSecrets && len(refs) != 0 {
			resolver, err := secrets.New(c.Service.SecretRoots, 64<<10)
			if err != nil {
				public := safeerr.Public(err)
				fmt.Fprintln(errOut, public.Error())
				return 2
			}
			defer resolver.Close()
			for _, ref := range refs {
				data, err := resolver.Read(ctx, ref)
				clear(data)
				if err != nil {
					public := safeerr.Public(err)
					fmt.Fprintln(errOut, public.Error())
					return 2
				}
			}
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

// runControl parses only the documented command grammar; flag diagnostics and
// rejected argument values never reach operator output.
func runControl(ctx context.Context, args []string, out, errOut io.Writer) int {
	invalid := func() int {
		fmt.Fprintln(errOut, (&safeerr.Error{Code: safeerr.Invalid, Field: "command"}).Error())
		return 2
	}
	method, path := "GET", ""
	rest := args[1:]
	positionals := 0
	switch args[0] {
	case "health":
		path = "/v1/health"
	case "jobs":
		if len(rest) == 0 || rest[0] != "list" {
			return invalid()
		}
		rest = rest[1:]
		path = "/v1/jobs"
	case "history":
		if len(rest) == 0 {
			return invalid()
		}
		switch rest[0] {
		case "list":
			path = "/v1/runs"
		case "show":
			path = "/v1/runs/"
			positionals = 1
		default:
			return invalid()
		}
		rest = rest[1:]
	case "operation":
		if len(rest) == 0 || rest[0] != "cancel" {
			return invalid()
		}
		rest = rest[1:]
		method = "POST"
		path = "/v1/runs/"
		positionals = 1
	}
	flags := flag.NewFlagSet("control", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	socket := flags.String("socket", "/run/bkplight/control.sock", "local Unix socket")
	var limit *int
	var after, requestID *string
	if path == "/v1/runs" {
		limit = flags.Int("limit", 50, "history page limit")
		after = flags.String("after", "", "history cursor")
	}
	if method == "POST" {
		requestID = flags.String("request-id", "", "idempotency request ID")
	}
	// The documented RUN positional precedes --socket/--request-id. flag.Parse
	// stops at a positional, so move validated flag tokens before positionals.
	var flagArgs, values []string
	seen := map[string]bool{}
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		if !strings.HasPrefix(arg, "-") {
			values = append(values, arg)
			continue
		}
		name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
		if name == "" || seen[name] || flags.Lookup(name) == nil {
			return invalid()
		}
		seen[name] = true
		flagArgs = append(flagArgs, arg)
		if !strings.Contains(arg, "=") {
			i++
			if i >= len(rest) {
				return invalid()
			}
			flagArgs = append(flagArgs, rest[i])
		}
	}
	if err := flags.Parse(append(flagArgs, values...)); err != nil || flags.NArg() != positionals {
		return invalid()
	}
	if positionals == 1 {
		id := flags.Arg(0)
		if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(id) {
			return invalid()
		}
		path += id
		if method == "POST" {
			path += "/cancel"
		}
	}
	if limit != nil {
		if *limit < 1 || *limit > 200 {
			return invalid()
		}
		q := url.Values{"limit": []string{strconv.Itoa(*limit)}}
		if *after != "" {
			q.Set("after", *after)
		}
		path += "?" + q.Encode()
	}
	var body []byte
	if requestID != nil {
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`).MatchString(*requestID) {
			return invalid()
		}
		body, _ = json.Marshal(struct {
			RequestID string `json:"request_id"`
		}{*requestID})
	}
	response, err := control.NewClient(*socket).Do(ctx, method, path, body)
	if len(response) > 0 {
		fmt.Fprintln(out, string(response))
	}
	if err == nil {
		return 0
	}
	public := safeerr.Public(err)
	fmt.Fprintln(errOut, public.Error())
	switch public.Code {
	case safeerr.Invalid:
		return 2
	case safeerr.NotImplemented:
		return 4
	default:
		return 3
	}
}
