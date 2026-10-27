package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	pal "github.com/glycerine/apalache-go"
)

const (
	exitOK       = 0
	exitTool     = 1
	exitUsage    = 2
	exitFrontend = 4
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return exitOK
	case "version", "--version":
		fmt.Fprintln(stdout, version)
		return exitOK
	case "parse":
		return runParse(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return exitUsage
	}
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, string(os.PathListSeparator)) }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func runParse(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("parse", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "summary", "output format: summary or json")
	includeSource := flags.Bool("source", false, "include source positions in JSON IR")
	var libraryPaths stringList
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "parse requires exactly one TLA+ file")
		return exitUsage
	}
	if *output != "summary" && *output != "json" {
		fmt.Fprintf(stderr, "invalid output format %q (want summary or json)\n", *output)
		return exitUsage
	}

	result := pal.ParseFile(ctx, flags.Arg(0), pal.FrontendOptions{
		LibraryPaths:  libraryPaths,
		IncludeSource: *includeSource,
	})
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !result.OK() {
		return exitFrontend
	}

	if *output == "json" {
		var formatted any
		if err := json.Unmarshal(result.IR, &formatted); err != nil {
			fmt.Fprintf(stderr, "cannot decode frontend IR: %v\n", err)
			return exitTool
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(formatted); err != nil {
			fmt.Fprintf(stderr, "cannot write frontend IR: %v\n", err)
			return exitTool
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "parsed module %s\n", result.Module)
	return exitOK
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: apalache <command> [options]")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  parse FILE    parse and semantically check TLA+")
	fmt.Fprintln(w, "  version       print build version")
}
