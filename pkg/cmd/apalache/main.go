package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"

	pal "github.com/glycerine/apalache-go"
)

const (
	exitOK             = 0
	exitTool           = 1
	exitUsage          = 2
	exitFrontend       = 4
	exitCounterexample = 12
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
	case "typecheck":
		return runTypeCheck(ctx, args[1:], stdout, stderr)
	case "check":
		return runCheck(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return exitUsage
	}
}

func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	length := flags.Int("length", 10, "maximum number of Next steps")
	configPath := flags.String("config", "", "TLC configuration file")
	initName := flags.String("init", "", "initial-state operator")
	nextName := flags.String("next", "", "next-state operator")
	constantInit := flags.String("cinit", "", "constant initializer operator")
	useRows := flags.Bool("rows", false, "enable row types")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var invariants stringList
	var libraryPaths stringList
	flags.Var(&invariants, "inv", "state invariant operator (repeatable)")
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "check requires exactly one TLA+ file")
		return exitUsage
	}
	if *length < 0 {
		fmt.Fprintln(stderr, "check length cannot be negative")
		return exitUsage
	}

	configuration := pal.SpecificationConfiguration{
		ConstantInit: *constantInit,
		Invariants:   append([]string(nil), invariants...),
	}
	if *configPath != "" {
		contents, err := os.ReadFile(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "cannot read TLC configuration: %v\n", err)
			return exitFrontend
		}
		parsed, err := pal.ParseTLCConfig(string(contents))
		if err != nil {
			fmt.Fprintf(stderr, "cannot parse TLC configuration: %v\n", err)
			return exitFrontend
		}
		configuration.TLCConfig = &parsed
		configuration.ContextName = *configPath
	}
	if (*initName == "") != (*nextName == "") {
		fmt.Fprintln(stderr, "-init and -next must be provided together")
		return exitUsage
	}
	if *initName != "" {
		configuration.Behavior = pal.InitNextSpec{Init: *initName, Next: *nextName}
	} else if configuration.TLCConfig == nil {
		fmt.Fprintln(stderr, "check requires either -config or both -init and -next")
		return exitUsage
	}

	frontend := pal.ParseAndTypeCheckFile(ctx, flags.Arg(0), pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: *useRows, DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range frontend.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !frontend.OK() {
		return exitFrontend
	}
	prepared, err := pal.PrepareSafetyTransitions(frontend.Native, pal.SafetyPipelineOptions{
		Specification: configuration,
		Sources:       frontend.Sources,
	})
	if err != nil {
		fmt.Fprintf(stderr, "cannot prepare safety transitions: %v\n", err)
		return exitTool
	}
	if len(prepared.Predicates.Invariants) == 0 {
		fmt.Fprintln(stderr, "check requires at least one invariant")
		return exitUsage
	}
	result, err := pal.CheckBoundedSafety(ctx, prepared.Module, prepared.Predicates, *length)
	if err != nil {
		fmt.Fprintf(stderr, "bounded safety check failed: %v\n", err)
		return exitTool
	}
	switch result.Outcome {
	case pal.SafetyCounterexample:
		fmt.Fprintf(stdout, "Invariant %s violated at depth %d (Init$%04d", result.Invariant, result.Depth, result.InitTransition)
		for _, number := range result.NextTransitions {
			fmt.Fprintf(stdout, ", Next$%04d", number)
		}
		fmt.Fprintln(stdout, ").")
		for index, state := range result.States {
			names := make([]string, 0, len(state.Values))
			for name := range state.Values {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				value, formatErr := pal.FormatTLAExpression(state.Values[name])
				if formatErr != nil {
					fmt.Fprintf(stderr, "cannot format counterexample state: %v\n", formatErr)
					return exitTool
				}
				fmt.Fprintf(stdout, "State %d: %s = %s\n", index, name, value)
			}
		}
		return exitCounterexample
	case pal.SafetyNoCounterexample:
		fmt.Fprintf(stdout, "No counterexample found through depth %d (%d symbolic executions).\n", *length, result.ExploredExecutions)
		return exitOK
	default:
		fmt.Fprintln(stderr, "solver returned unknown")
		return exitTool
	}
}

func runTypeCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("typecheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "summary", "output format: summary or json")
	includeSource := flags.Bool("source", false, "include source positions in JSON IR")
	useRows := flags.Bool("rows", false, "enable row types")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var libraryPaths stringList
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "typecheck requires exactly one TLA+ file")
		return exitUsage
	}
	if *output != "summary" && *output != "json" {
		fmt.Fprintf(stderr, "invalid output format %q (want summary or json)\n", *output)
		return exitUsage
	}

	result := pal.ParseAndTypeCheckFile(ctx, flags.Arg(0), pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: *useRows, DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !result.OK() {
		return exitFrontend
	}
	if *output == "json" {
		options := pal.IRJSONOptions{}
		if *includeSource {
			options.SourceStore = result.Sources
		}
		encoded, err := pal.MarshalSingleModuleIRJSON(result.Native, options)
		if err != nil {
			fmt.Fprintf(stderr, "cannot encode typed IR: %v\n", err)
			return exitTool
		}
		var formatted any
		if err = json.Unmarshal(encoded, &formatted); err != nil {
			fmt.Fprintf(stderr, "cannot decode typed IR: %v\n", err)
			return exitTool
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err = encoder.Encode(formatted); err != nil {
			fmt.Fprintf(stderr, "cannot write typed IR: %v\n", err)
			return exitTool
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "type-checked module %s\n", result.Module)
	return exitOK
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
	fmt.Fprintln(w, "  check FILE    bounded safety check")
	fmt.Fprintln(w, "  parse FILE    parse and semantically check TLA+")
	fmt.Fprintln(w, "  typecheck FILE parse and type-check annotated TLA+")
	fmt.Fprintln(w, "  version       print build version")
}
