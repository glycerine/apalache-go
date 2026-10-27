package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "simulate":
		return runSimulate(ctx, args[1:], stdout, stderr)
	case "test":
		return runTest(ctx, args[1:], stdout, stderr)
	case "tracee":
		return runTracee(ctx, args[1:], stdout, stderr)
	case "transpile":
		return runTranspile(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return exitUsage
	}
}

func runTranspile(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("transpile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "TLC configuration file")
	initName := flags.String("init", "", "initial-state operator")
	nextName := flags.String("next", "", "next-state operator")
	constantInit := flags.String("cinit", "", "constant initializer operator")
	outputPath := flags.String("output", "output.vmt", "write VMT constraints to this file, or - for stdout")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var invariants, temporalProperties csvStringList
	var libraryPaths stringList
	flags.Var(&invariants, "inv", "state invariant operator (repeatable or comma-separated)")
	flags.Var(&temporalProperties, "temporal", "temporal property operator (repeatable or comma-separated)")
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "transpile requires exactly one TLA+ file")
		return exitUsage
	}
	if *outputPath == "" {
		fmt.Fprintln(stderr, "transpile output path cannot be empty")
		return exitUsage
	}
	rows, featureErr := resolveRowsFeature(*useRows, *features)
	if featureErr != nil {
		fmt.Fprintln(stderr, featureErr)
		return exitUsage
	}
	configuration := pal.SpecificationConfiguration{
		ConstantInit:       *constantInit,
		Invariants:         append([]string(nil), invariants...),
		TemporalProperties: append([]string(nil), temporalProperties...),
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
		configuration.Behavior = pal.InitNextSpec{Init: "Init", Next: "Next"}
	}
	frontend := pal.ParseAndTypeCheckFile(ctx, flags.Arg(0), pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, DisablePolytypes: *disablePolytypes})
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
		fmt.Fprintf(stderr, "cannot prepare VMT transitions: %v\n", err)
		return exitTool
	}
	document, err := pal.FormatVMTModuleWithInvariants(prepared.Module, prepared.Predicates.Invariants)
	if err != nil {
		fmt.Fprintf(stderr, "cannot generate VMT constraints: %v\n", err)
		return exitTool
	}
	if *outputPath == "-" {
		if _, err = io.WriteString(stdout, document); err != nil {
			fmt.Fprintf(stderr, "cannot write VMT constraints: %v\n", err)
			return exitTool
		}
		return exitOK
	}
	if err = os.WriteFile(*outputPath, []byte(document), 0o644); err != nil {
		fmt.Fprintf(stderr, "cannot write VMT constraints: %v\n", err)
		return exitTool
	}
	absolute, absoluteErr := filepath.Abs(*outputPath)
	if absoluteErr != nil {
		absolute = *outputPath
	}
	fmt.Fprintf(stdout, "VMT constraints successfully generated at\n%s\n", absolute)
	return exitOK
}

func runTracee(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tracee", flag.ContinueOnError)
	flags.SetOutput(stderr)
	tracePath := flags.String("trace", "", "input ITF trace")
	itfPath := flags.String("itf", "", "write evaluated ITF trace to this file (default stdout)")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var expressions csvStringList
	var libraryPaths stringList
	flags.Var(&expressions, "expressions", "nullary operators to evaluate (repeatable or comma-separated)")
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	var sourcePath string
	switch flags.NArg() {
	case 1:
		sourcePath = flags.Arg(0)
	case 3:
		if *tracePath != "" || len(expressions) != 0 {
			fmt.Fprintln(stderr, "tracee cannot mix positional trace arguments with -trace or -expressions")
			return exitUsage
		}
		sourcePath, *tracePath = flags.Arg(0), flags.Arg(1)
		if err := expressions.Set(flags.Arg(2)); err != nil {
			fmt.Fprintf(stderr, "invalid trace expression list: %v\n", err)
			return exitUsage
		}
	default:
		fmt.Fprintln(stderr, "tracee requires SOURCE TRACE EXPRESSIONS, or -trace and -expressions with SOURCE")
		return exitUsage
	}
	if *tracePath == "" || len(expressions) == 0 {
		fmt.Fprintln(stderr, "tracee requires a trace and at least one expression operator")
		return exitUsage
	}
	rows, featureErr := resolveRowsFeature(*useRows, *features)
	if featureErr != nil {
		fmt.Fprintln(stderr, featureErr)
		return exitUsage
	}
	frontend := pal.ParseAndTypeCheckFile(ctx, sourcePath, pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range frontend.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !frontend.OK() {
		return exitFrontend
	}
	traceData, err := os.ReadFile(*tracePath)
	if err != nil {
		fmt.Fprintf(stderr, "cannot read ITF trace: %v\n", err)
		return exitFrontend
	}
	parameterTypes := make(map[string]pal.Type)
	for _, declaration := range frontend.Native.ConstDeclarations() {
		if typ, typed := declaration.Tag().Type(); typed {
			parameterTypes[declaration.Name()] = typ
		}
	}
	states, err := pal.UnmarshalITFTraceWithParams(traceData, parameterTypes)
	if err != nil {
		fmt.Fprintf(stderr, "cannot decode ITF trace: %v\n", err)
		return exitFrontend
	}
	evaluation, err := pal.EvaluateTrace(ctx, frontend.Native, states, expressions)
	if err != nil {
		fmt.Fprintf(stderr, "trace evaluation failed: %v\n", err)
		return exitTool
	}
	outputStates := make([]pal.TraceState, 1, len(evaluation.States)+1)
	outputStates = append(outputStates, evaluation.States...)
	data, err := pal.MarshalITFTrace(evaluation.Module, outputStates, pal.ITFOptions{
		Description: "Trace expressions evaluated by Apalache Go",
	})
	if err != nil {
		fmt.Fprintf(stderr, "cannot encode evaluated ITF trace: %v\n", err)
		return exitTool
	}
	data = append(data, '\n')
	if *itfPath == "" {
		if _, err = stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "cannot write evaluated trace: %v\n", err)
			return exitTool
		}
	} else {
		if err = os.WriteFile(*itfPath, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "cannot write evaluated trace: %v\n", err)
			return exitTool
		}
		fmt.Fprintf(stdout, "Wrote evaluated ITF trace to %s.\n", *itfPath)
	}
	return exitOK
}

func runConfig(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("config", flag.ContinueOnError)
	flags.SetOutput(stderr)
	enableStats := flags.String("enable-stats", "", "enable shared TLA+ usage statistics: true or false (default unchanged)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "config does not accept positional arguments")
		return exitUsage
	}
	if *enableStats != "" && *enableStats != "true" && *enableStats != "false" {
		fmt.Fprintln(stderr, "-enable-stats must be true or false")
		return exitUsage
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		fmt.Fprintf(stderr, "cannot find the user home directory: %v\n", err)
		return exitTool
	}
	configDir := filepath.Join(home, ".tlaplus")
	if err = os.MkdirAll(configDir, 0o700); err != nil {
		fmt.Fprintf(stderr, "cannot create TLA+ configuration directory: %v\n", err)
		return exitTool
	}
	if *enableStats != "" {
		contents := "NO_STATISTICS"
		if *enableStats == "true" {
			identifier := make([]byte, 16)
			if _, err = rand.Read(identifier); err != nil {
				fmt.Fprintf(stderr, "cannot generate statistics identifier: %v\n", err)
				return exitTool
			}
			contents = hex.EncodeToString(identifier)
		}
		if err = os.WriteFile(filepath.Join(configDir, "esc.txt"), []byte(contents+"\n"), 0o600); err != nil {
			fmt.Fprintf(stderr, "cannot update statistics configuration: %v\n", err)
			return exitTool
		}
	}
	fmt.Fprintln(stdout, "Configuration complete")
	return exitOK
}

func runTest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(stderr)
	constantInit := flags.String("cinit", "", "constant initializer operator")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var libraryPaths stringList
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 4 {
		fmt.Fprintln(stderr, "test requires FILE BEFORE ACTION ASSERTION")
		return exitUsage
	}
	path, before, action, assertion := flags.Arg(0), flags.Arg(1), flags.Arg(2), flags.Arg(3)
	rows, featureErr := resolveRowsFeature(*useRows, *features)
	if featureErr != nil {
		fmt.Fprintln(stderr, featureErr)
		return exitUsage
	}
	frontend := pal.ParseAndTypeCheckFile(ctx, path, pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range frontend.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !frontend.OK() {
		return exitFrontend
	}
	prepared, err := pal.PrepareSafetyTransitions(frontend.Native, pal.SafetyPipelineOptions{
		Specification: pal.SpecificationConfiguration{
			Behavior: pal.InitNextSpec{Init: before, Next: action}, ConstantInit: *constantInit,
			Invariants: []string{assertion},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		fmt.Fprintf(stderr, "cannot prepare test: %v\n", err)
		return exitTool
	}
	result, err := pal.CheckBoundedSafetyWithOptions(ctx, prepared.Module, prepared.Predicates,
		pal.BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: true, InvariantMinimumDepth: 1})
	if err != nil {
		fmt.Fprintf(stderr, "test failed: %v\n", err)
		return exitTool
	}
	switch result.Outcome {
	case pal.SafetyNoCounterexample:
		fmt.Fprintln(stdout, "No example found.")
		return exitOK
	case pal.SafetyCounterexample:
		fmt.Fprintf(stdout, "Assertion %s violated after %s.\n", assertion, action)
		if err = printDecodedStates(stdout, prepared.Module, result.States); err != nil {
			fmt.Fprintf(stderr, "cannot format test counterexample: %v\n", err)
			return exitTool
		}
		return exitCounterexample
	case pal.SafetyDeadlock:
		fmt.Fprintf(stdout, "Action %s is disabled after %s.\n", action, before)
		if err = printDecodedStates(stdout, prepared.Module, result.States); err != nil {
			fmt.Fprintf(stderr, "cannot format test deadlock: %v\n", err)
			return exitTool
		}
		return exitCounterexample
	default:
		fmt.Fprintln(stderr, "solver returned unknown")
		return exitTool
	}
}

func runSimulate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("simulate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	backend := addCheckerBackendFlags(flags)
	length := flags.Int("length", 10, "maximum number of Next steps")
	timeoutSMT := flags.Int("timeout-smt", 0, "limit each SMT query to this many seconds (0 is unlimited)")
	configPath := flags.String("config", "", "TLC configuration file")
	initName := flags.String("init", "", "initial-state operator")
	nextName := flags.String("next", "", "next-state operator")
	constantInit := flags.String("cinit", "", "constant initializer operator")
	itfPath := flags.String("itf", "", "write the simulated trace as ITF JSON")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var libraryPaths stringList
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "simulate requires exactly one TLA+ file")
		return exitUsage
	}
	if *length < 0 {
		fmt.Fprintln(stderr, "simulate length cannot be negative")
		return exitUsage
	}
	if err := validateSMTTimeout(*timeoutSMT); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if err := validateCheckerBackendOptions(backend); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	ctx = pal.WithSMTQueryTimeout(ctx, time.Duration(*timeoutSMT)*time.Second)
	rows, featureErr := resolveRowsFeature(*useRows, *features)
	if featureErr != nil {
		fmt.Fprintln(stderr, featureErr)
		return exitUsage
	}
	configuration := pal.SpecificationConfiguration{ConstantInit: *constantInit}
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
		configuration.Behavior = pal.InitNextSpec{Init: "Init", Next: "Next"}
	}

	frontend := pal.ParseAndTypeCheckFile(ctx, flags.Arg(0), pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range frontend.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !frontend.OK() {
		return exitFrontend
	}
	prepared, err := pal.PrepareSafetyTransitions(frontend.Native, pal.SafetyPipelineOptions{
		Specification: configuration, Sources: frontend.Sources,
	})
	if err != nil {
		fmt.Fprintf(stderr, "cannot prepare simulation: %v\n", err)
		return exitTool
	}
	result, err := pal.SimulateBehavior(ctx, prepared.Module, prepared.Predicates, *length)
	if err != nil {
		fmt.Fprintf(stderr, "simulation failed: %v\n", err)
		return exitTool
	}
	if result.Outcome == pal.SimulationUnknown {
		fmt.Fprintln(stderr, "solver returned unknown")
		return exitTool
	}
	if *itfPath != "" {
		data, encodeErr := pal.MarshalDecodedTraceITF(prepared.Module, result.States, pal.ITFOptions{})
		if encodeErr != nil {
			fmt.Fprintf(stderr, "cannot encode ITF simulation: %v\n", encodeErr)
			return exitTool
		}
		if writeErr := os.WriteFile(*itfPath, append(data, '\n'), 0o644); writeErr != nil {
			fmt.Fprintf(stderr, "cannot write ITF simulation: %v\n", writeErr)
			return exitTool
		}
		fmt.Fprintf(stdout, "Wrote ITF trace to %s.\n", *itfPath)
	}
	if result.Outcome == pal.SimulationDeadlock {
		fmt.Fprintf(stdout, "Simulation deadlocked at depth %d.\n", result.Depth)
	} else {
		fmt.Fprintf(stdout, "Simulation completed through depth %d.\n", result.Depth)
	}
	if err = printDecodedStates(stdout, prepared.Module, result.States); err != nil {
		fmt.Fprintf(stderr, "cannot format simulated state: %v\n", err)
		return exitTool
	}
	return exitOK
}

func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	backend := addCheckerBackendFlags(flags)
	length := flags.Int("length", 10, "maximum number of Next steps")
	timeoutSMT := flags.Int("timeout-smt", 0, "limit each SMT query to this many seconds (0 is unlimited)")
	configPath := flags.String("config", "", "TLC configuration file")
	initName := flags.String("init", "", "initial-state operator")
	nextName := flags.String("next", "", "next-state operator")
	constantInit := flags.String("cinit", "", "constant initializer operator")
	deadlockFlag := flags.String("deadlock", "", "check deadlocks: true or false (default true)")
	noDeadlock := flags.Bool("no-deadlock", false, "disable deadlock checking (Scala-compatible alias)")
	itfPath := flags.String("itf", "", "write a counterexample trace as ITF JSON")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var invariants, temporalProperties csvStringList
	var libraryPaths stringList
	flags.Var(&invariants, "inv", "state invariant operator (repeatable)")
	flags.Var(&temporalProperties, "temporal", "temporal property operator (repeatable or comma-separated)")
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
	if err := validateSMTTimeout(*timeoutSMT); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if err := validateCheckerBackendOptions(backend); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	ctx = pal.WithSMTQueryTimeout(ctx, time.Duration(*timeoutSMT)*time.Second)
	rows, featureErr := resolveRowsFeature(*useRows, *features)
	if featureErr != nil {
		fmt.Fprintln(stderr, featureErr)
		return exitUsage
	}

	configuration := pal.SpecificationConfiguration{
		ConstantInit:       *constantInit,
		Invariants:         append([]string(nil), invariants...),
		TemporalProperties: append([]string(nil), temporalProperties...),
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
		configuration.Behavior = pal.InitNextSpec{Init: "Init", Next: "Next"}
	}
	checkDeadlocks := true
	if configuration.TLCConfig != nil && configuration.TLCConfig.CheckDeadlock != nil {
		checkDeadlocks = *configuration.TLCConfig.CheckDeadlock
	}
	if *deadlockFlag != "" {
		parsed, err := strconv.ParseBool(*deadlockFlag)
		if err != nil {
			fmt.Fprintf(stderr, "invalid -deadlock value %q (want true or false)\n", *deadlockFlag)
			return exitUsage
		}
		checkDeadlocks = parsed
	}
	if *noDeadlock {
		if *deadlockFlag != "" && checkDeadlocks {
			fmt.Fprintln(stderr, "-no-deadlock conflicts with -deadlock=true")
			return exitUsage
		}
		checkDeadlocks = false
	}

	frontend := pal.ParseAndTypeCheckFile(ctx, flags.Arg(0), pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, DisablePolytypes: *disablePolytypes})
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
	if len(prepared.Predicates.Invariants) == 0 && !checkDeadlocks {
		fmt.Fprintln(stderr, "check requires at least one invariant when deadlock checking is disabled")
		return exitUsage
	}
	result, err := pal.CheckBoundedSafetyWithOptions(ctx, prepared.Module, prepared.Predicates, pal.BoundedSafetyOptions{
		MaxSteps: *length, CheckDeadlocks: checkDeadlocks,
	})
	if err != nil {
		fmt.Fprintf(stderr, "bounded safety check failed: %v\n", err)
		return exitTool
	}
	if *itfPath != "" && (result.Outcome == pal.SafetyCounterexample || result.Outcome == pal.SafetyDeadlock) {
		data, encodeErr := pal.MarshalDecodedTraceITF(prepared.Module, result.States, pal.ITFOptions{})
		if encodeErr != nil {
			fmt.Fprintf(stderr, "cannot encode ITF counterexample: %v\n", encodeErr)
			return exitTool
		}
		if writeErr := os.WriteFile(*itfPath, append(data, '\n'), 0o644); writeErr != nil {
			fmt.Fprintf(stderr, "cannot write ITF counterexample: %v\n", writeErr)
			return exitTool
		}
		fmt.Fprintf(stdout, "Wrote ITF trace to %s.\n", *itfPath)
	}
	switch result.Outcome {
	case pal.SafetyCounterexample:
		kind := "Invariant"
		if result.TraceInvariant {
			kind = "Trace invariant"
		} else if result.ActionInvariant {
			kind = "Action invariant"
		}
		fmt.Fprintf(stdout, "%s %s violated at depth %d (Init$%04d", kind, result.Invariant, result.Depth, result.InitTransition)
		for _, number := range result.NextTransitions {
			fmt.Fprintf(stdout, ", Next$%04d", number)
		}
		fmt.Fprintln(stdout, ").")
		if formatErr := printDecodedStates(stdout, prepared.Module, result.States); formatErr != nil {
			fmt.Fprintf(stderr, "cannot format counterexample state: %v\n", formatErr)
			return exitTool
		}
		return exitCounterexample
	case pal.SafetyDeadlock:
		fmt.Fprintf(stdout, "Deadlock found at depth %d.\n", result.Depth)
		if formatErr := printDecodedStates(stdout, prepared.Module, result.States); formatErr != nil {
			fmt.Fprintf(stderr, "cannot format deadlock state: %v\n", formatErr)
			return exitTool
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

func printDecodedStates(output io.Writer, module *pal.Module, states []pal.DecodedState) error {
	states = pal.UserDecodedStates(module, states)
	for index, state := range states {
		names := make([]string, 0, len(state.Values))
		for name := range state.Values {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			value, err := pal.FormatTLAExpression(state.Values[name])
			if err != nil {
				return err
			}
			fmt.Fprintf(output, "State %d: %s = %s\n", index, name, value)
		}
	}
	return nil
}

func runTypeCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("typecheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "summary", "output format (summary/json) or .tla/.json file")
	includeSource := flags.Bool("source", false, "include source positions in JSON IR")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
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
	if !validModuleOutput(*output) {
		fmt.Fprintf(stderr, "invalid output %q (want summary, json, or a .tla/.json file)\n", *output)
		return exitUsage
	}
	rows, featureErr := resolveRowsFeature(*useRows, *features)
	if featureErr != nil {
		fmt.Fprintln(stderr, featureErr)
		return exitUsage
	}

	result := pal.ParseAndTypeCheckFile(ctx, flags.Arg(0), pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !result.OK() {
		return exitFrontend
	}
	if *output == "json" || strings.EqualFold(filepath.Ext(*output), ".json") {
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
		if err = writePrettyJSONOutput(*output, formatted, stdout); err != nil {
			fmt.Fprintf(stderr, "cannot write typed IR: %v\n", err)
			return exitTool
		}
		return exitOK
	}
	if strings.EqualFold(filepath.Ext(*output), ".tla") {
		formatted, err := pal.FormatTLAModule(result.Native, pal.TLAFormatOptions{
			ExtendedModules: result.Extends, Annotations: result.Annotations,
		})
		if err != nil {
			fmt.Fprintf(stderr, "cannot format typed TLA+: %v\n", err)
			return exitTool
		}
		if err = os.WriteFile(*output, []byte(formatted), 0o644); err != nil {
			fmt.Fprintf(stderr, "cannot write typed TLA+: %v\n", err)
			return exitTool
		}
		fmt.Fprintf(stdout, "Wrote typed TLA+ to %s.\n", *output)
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

type csvStringList []string

func (s *csvStringList) String() string { return strings.Join(*s, ",") }
func (s *csvStringList) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return fmt.Errorf("operator name list contains an empty name")
		}
		*s = append(*s, item)
	}
	return nil
}

func runParse(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("parse", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "summary", "output format (summary/json) or .tla/.json file")
	includeSource := flags.Bool("source", false, "include source positions in JSON IR")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	var libraryPaths stringList
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "parse requires exactly one TLA+ file")
		return exitUsage
	}
	if !validModuleOutput(*output) {
		fmt.Fprintf(stderr, "invalid output %q (want summary, json, or a .tla/.json file)\n", *output)
		return exitUsage
	}
	if _, err := resolveRowsFeature(false, *features); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}

	result := pal.ParseFile(ctx, flags.Arg(0), pal.FrontendOptions{
		LibraryPaths:  libraryPaths,
		IncludeSource: *includeSource || strings.EqualFold(filepath.Ext(*output), ".tla"),
	})
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !result.OK() {
		return exitFrontend
	}

	if *output == "json" || strings.EqualFold(filepath.Ext(*output), ".json") {
		var formatted any
		if err := json.Unmarshal(result.IR, &formatted); err != nil {
			fmt.Fprintf(stderr, "cannot decode frontend IR: %v\n", err)
			return exitTool
		}
		if err := writePrettyJSONOutput(*output, formatted, stdout); err != nil {
			fmt.Fprintf(stderr, "cannot write frontend IR: %v\n", err)
			return exitTool
		}
		return exitOK
	}
	if strings.EqualFold(filepath.Ext(*output), ".tla") {
		source, err := os.ReadFile(flags.Arg(0))
		if err != nil {
			fmt.Fprintf(stderr, "cannot recover source annotations: %v\n", err)
			return exitFrontend
		}
		annotations := pal.ExtractTypeAnnotations(flags.Arg(0), string(source), result.Native, result.Sources)
		formatted, err := pal.FormatTLAModule(result.Native, pal.TLAFormatOptions{
			ExtendedModules: result.Extends, Annotations: annotations.Annotations,
		})
		if err != nil {
			fmt.Fprintf(stderr, "cannot format parsed TLA+: %v\n", err)
			return exitTool
		}
		if err = os.WriteFile(*output, []byte(formatted), 0o644); err != nil {
			fmt.Fprintf(stderr, "cannot write parsed TLA+: %v\n", err)
			return exitTool
		}
		fmt.Fprintf(stdout, "Wrote parsed TLA+ to %s.\n", *output)
		return exitOK
	}
	fmt.Fprintf(stdout, "parsed module %s\n", result.Module)
	return exitOK
}

func validModuleOutput(output string) bool {
	return output == "summary" || output == "json" ||
		strings.EqualFold(filepath.Ext(output), ".tla") || strings.EqualFold(filepath.Ext(output), ".json")
}

func resolveRowsFeature(rows bool, features string) (bool, error) {
	switch features {
	case "":
		return rows, nil
	case "rows":
		return true, nil
	case "no-rows":
		if rows {
			return false, fmt.Errorf("-rows conflicts with -features=no-rows")
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported feature %q (want rows or no-rows)", features)
	}
}

type checkerBackendOptions struct {
	algorithm string
	encoding  string
	solver    string
}

func addCheckerBackendFlags(flags *flag.FlagSet) *checkerBackendOptions {
	options := &checkerBackendOptions{}
	flags.StringVar(&options.algorithm, "algo", "incremental", "checking algorithm (incremental)")
	flags.StringVar(&options.encoding, "smt-encoding", "oopsla19", "SMT encoding (oopsla19)")
	flags.StringVar(&options.solver, "smt-solver", "z3", "SMT solver backend (z3 via g3)")
	return options
}

func validateCheckerBackendOptions(options *checkerBackendOptions) error {
	if options == nil {
		return fmt.Errorf("checker backend options are missing")
	}
	if algorithm := strings.ToLower(options.algorithm); algorithm != "incremental" {
		return fmt.Errorf("unsupported checker algorithm %q (Go port currently supports incremental)", options.algorithm)
	}
	if encoding := strings.ToLower(options.encoding); encoding != "oopsla19" {
		return fmt.Errorf("unsupported SMT encoding %q (Go port currently supports oopsla19)", options.encoding)
	}
	if solver := strings.ToLower(options.solver); solver != "z3" {
		return fmt.Errorf("unsupported SMT solver %q (Go port uses g3 v0.0.5 as its Z3 backend)", options.solver)
	}
	return nil
}

func validateSMTTimeout(seconds int) error {
	const maxDuration = time.Duration(1<<63 - 1)
	if seconds < 0 {
		return fmt.Errorf("SMT timeout cannot be negative")
	}
	if int64(seconds) > int64(maxDuration/time.Second) {
		return fmt.Errorf("SMT timeout is too large")
	}
	return nil
}

func writePrettyJSONOutput(output string, value any, stdout io.Writer) error {
	var destination io.Writer = stdout
	var file *os.File
	if output != "json" {
		var err error
		file, err = os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer file.Close()
		destination = file
	}
	encoder := json.NewEncoder(destination)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: apalache <command> [options]")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  check FILE    bounded safety check")
	fmt.Fprintln(w, "  config        configure shared TLA+ options")
	fmt.Fprintln(w, "  parse FILE    parse and semantically check TLA+")
	fmt.Fprintln(w, "  simulate FILE generate one bounded behavior")
	fmt.Fprintln(w, "  test FILE BEFORE ACTION ASSERTION run a one-step test")
	fmt.Fprintln(w, "  tracee SOURCE TRACE EXPRESSIONS evaluate operators over an ITF trace")
	fmt.Fprintln(w, "  transpile FILE transpile ReTLA to VMT constraints")
	fmt.Fprintln(w, "  typecheck FILE parse and type-check annotated TLA+")
	fmt.Fprintln(w, "  version       print build version")
}
