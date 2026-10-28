package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	pal "github.com/glycerine/apalache-go"
	"github.com/glycerine/apalache-go/shai"
	"google.golang.org/grpc"
)

const (
	exitOK             = 0
	exitTool           = 1
	exitUsage          = 2
	exitFrontend       = 4
	exitCounterexample = 12
)

var version = "dev"
var listenTCP = net.Listen

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
	case "server":
		return runServer(ctx, args[1:], stdout, stderr)
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

func runServer(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	port := flags.Int("port", 8822, "port served by the RPC server")
	serverType := flags.String("server-type", "explorer", "server type: explorer or checker")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "server does not accept positional arguments")
		return exitUsage
	}
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	if selectedConfig != "" {
		if err = applicationConfig.validateForServer(); err != nil {
			fmt.Fprintf(stderr, "invalid JSON server configuration: %v\n", err)
			return exitUsage
		}
		if !visited["port"] && applicationConfig.Server.Port != nil {
			*port = *applicationConfig.Server.Port
		}
		if !visited["server-type"] && applicationConfig.Server.ServerType != nil {
			*serverType = *applicationConfig.Server.ServerType
		}
	}
	if !visited["port"] {
		if environmentPort, found := os.LookupEnv("PORT"); found && environmentPort != "" {
			parsedPort, parseErr := strconv.Atoi(environmentPort)
			if parseErr != nil {
				fmt.Fprintf(stderr, "invalid PORT environment value %q: %v\n", environmentPort, parseErr)
				return exitUsage
			}
			*port = parsedPort
		}
	}
	if *port < 0 || *port > 65535 {
		fmt.Fprintln(stderr, "server port must be in [0, 65535]")
		return exitUsage
	}
	kind := strings.ToLower(*serverType)
	if kind != "explorer" && kind != "checker" {
		fmt.Fprintf(stderr, "unsupported server type %q (want explorer or checker)\n", *serverType)
		return exitUsage
	}
	listener, err := listenTCP("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		fmt.Fprintf(stderr, "cannot listen for %s server: %v\n", kind, err)
		return exitTool
	}
	if kind == "checker" {
		return serveChecker(ctx, listener, stdout, stderr)
	}
	server := newExplorationHTTPServer(ctx)
	fmt.Fprintf(stdout, "JSON-RPC explorer server running on http://localhost:%d/rpc (gzip+zstd compression enabled).\n", listener.Addr().(*net.TCPAddr).Port)
	if ctx.Err() != nil {
		_ = listener.Close()
		return exitOK
	}
	shutdownContext, stopShutdown := context.WithCancel(ctx)
	defer stopShutdown()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-shutdownContext.Done()
		if ctx.Err() != nil {
			graceContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := server.Shutdown(graceContext); err != nil {
				_ = server.Close()
			}
		}
	}()
	err = server.Serve(listener)
	stopShutdown()
	<-shutdownDone
	if err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(stderr, "JSON-RPC server failed: %v\n", err)
		return exitTool
	}
	return exitOK
}

func serveChecker(ctx context.Context, listener net.Listener, stdout, stderr io.Writer) int {
	server := shai.NewServer()
	fmt.Fprintf(stdout, "gRPC checker server running on localhost:%d.\n", listener.Addr().(*net.TCPAddr).Port)
	if ctx.Err() != nil {
		_ = listener.Close()
		return exitOK
	}
	shutdownContext, stopShutdown := context.WithCancel(ctx)
	defer stopShutdown()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-shutdownContext.Done()
		if ctx.Err() == nil {
			return
		}
		graceful := make(chan struct{})
		go func() {
			server.GracefulStop()
			close(graceful)
		}()
		select {
		case <-graceful:
		case <-time.After(10 * time.Second):
			server.Stop()
			<-graceful
		}
	}()
	err := server.Serve(listener)
	stopShutdown()
	<-shutdownDone
	if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		fmt.Fprintf(stderr, "gRPC checker server failed: %v\n", err)
		return exitTool
	}
	return exitOK
}

func newExplorationHTTPServer(ctx context.Context) *http.Server {
	return &http.Server{
		Handler:           pal.NewExplorationJSONRPCHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
}

func runTranspile(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("transpile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	outDir := flags.String("out-dir", "", "root directory for source-namespaced run output")
	runDir := flags.String("run-dir", "", "additional directory receiving run output directly")
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
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	if selectedConfig != "" {
		if err = applicationConfig.validateForFrontend("transpile"); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if err = applyJSONFrontendDefaults(applicationConfig, visited, outputPath, features, useRows, disablePolytypes); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if err = applyJSONModelDefaults(applicationConfig, visited, jsonModelConfigTargets{
			TLCConfig: configPath, constantInit: constantInit, init: initName, next: nextName,
			invariants: &invariants, temporalProperties: &temporalProperties,
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
	}
	resolveOutputDirectoryOptions(applicationConfig, visited, outDir, runDir)
	if visited["init"] != visited["next"] {
		fmt.Fprintln(stderr, "-init and -next must be provided together")
		return exitUsage
	}
	if flags.NArg() > 1 || (flags.NArg() == 0 && applicationConfig.Source == nil) {
		fmt.Fprintln(stderr, "transpile requires one TLA+ file from the CLI or JSON configuration")
		return exitUsage
	}
	sourcePath := ""
	if flags.NArg() == 1 {
		sourcePath = flags.Arg(0)
	} else {
		var cleanup func()
		sourcePath, cleanup, err = applicationConfig.Source.materializeSpecification()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		defer cleanup()
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
	outputPaths, err := prepareCommandOutputPaths(*outDir, *runDir, sourcePath, "transpile")
	if err != nil {
		fmt.Fprintf(stderr, "cannot configure transpile output: %v\n", err)
		return exitUsage
	}
	if err = outputPaths.writeRunFile("transpile", args); err != nil {
		fmt.Fprintf(stderr, "cannot write transpile run metadata: %v\n", err)
		return exitTool
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
	outputFiles := []string{*outputPath}
	if !visited["output"] && applicationConfig.Output == nil {
		outputFiles = outputPaths.implicitFiles(*outputPath)
	}
	for _, path := range outputFiles {
		if err = os.WriteFile(path, []byte(document), 0o644); err != nil {
			fmt.Fprintf(stderr, "cannot write VMT constraints: %v\n", err)
			return exitTool
		}
		absolute, absoluteErr := filepath.Abs(path)
		if absoluteErr != nil {
			absolute = path
		}
		fmt.Fprintf(stdout, "VMT constraints successfully generated at\n%s\n", absolute)
	}
	return exitOK
}

func runTracee(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tracee", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
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
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	var sourcePath string
	switch flags.NArg() {
	case 0:
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
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	if selectedConfig != "" {
		if err = applicationConfig.validateForTracee(); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if err = applyJSONFrontendDefaults(applicationConfig, visited, nil, features, useRows, disablePolytypes); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if sourcePath == "" && applicationConfig.Source != nil {
			var cleanup func()
			sourcePath, cleanup, err = applicationConfig.Source.materializeSpecification()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return exitUsage
			}
			defer cleanup()
		}
		if !visited["trace"] && flags.NArg() != 3 {
			if configuredTrace, cleanup, found, traceErr := applicationConfig.configuredTracePath(); traceErr != nil {
				fmt.Fprintln(stderr, traceErr)
				return exitUsage
			} else if found {
				*tracePath = configuredTrace
				defer cleanup()
			}
		}
		if !visited["expressions"] && flags.NArg() != 3 && applicationConfig.TraceEvaluation.Expressions != nil {
			expressions = append(expressions[:0], (*applicationConfig.TraceEvaluation.Expressions)...)
		}
	}
	if sourcePath == "" || *tracePath == "" || len(expressions) == 0 {
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
	var states []pal.TraceState
	if inferSourceFormat(*tracePath) == "json" {
		states, err = pal.UnmarshalApalacheJSONTrace(traceData)
	} else {
		states, err = pal.UnmarshalITFTraceWithParams(traceData, parameterTypes)
	}
	if err != nil {
		fmt.Fprintf(stderr, "cannot decode trace: %v\n", err)
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
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	outDir := flags.String("out-dir", "", "root directory for source-namespaced run output")
	runDir := flags.String("run-dir", "", "additional directory receiving run output directly")
	constantInit := flags.String("cinit", "", "constant initializer operator")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var libraryPaths stringList
	checkerTuning := pal.CheckerTuning{}
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	if selectedConfig != "" {
		if err = applicationConfig.validateForModelCommand("test", false); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if err = applyJSONModelDefaults(applicationConfig, visited, jsonModelConfigTargets{
			constantInit: constantInit, useRows: useRows, disablePolytypes: disablePolytypes,
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if applicationConfig.Checker.Tuning != nil {
			checkerTuning, err = pal.ParseCheckerTuning(*applicationConfig.Checker.Tuning)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return exitUsage
			}
		}
	}
	resolveOutputDirectoryOptions(applicationConfig, visited, outDir, runDir)
	var path, before, action, assertion string
	switch {
	case flags.NArg() == 4:
		path, before, action, assertion = flags.Arg(0), flags.Arg(1), flags.Arg(2), flags.Arg(3)
	case flags.NArg() == 3 && applicationConfig.Source != nil:
		before, action, assertion = flags.Arg(0), flags.Arg(1), flags.Arg(2)
		var cleanup func()
		path, cleanup, err = applicationConfig.Source.materializeSpecification()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		defer cleanup()
	default:
		fmt.Fprintln(stderr, "test requires FILE BEFORE ACTION ASSERTION, or BEFORE ACTION ASSERTION with a JSON source")
		return exitUsage
	}
	outputPaths, err := prepareCommandOutputPaths(*outDir, *runDir, path, "test")
	if err != nil {
		fmt.Fprintf(stderr, "cannot configure test output: %v\n", err)
		return exitUsage
	}
	if err = outputPaths.writeRunFile("test", args); err != nil {
		fmt.Fprintf(stderr, "cannot write test run metadata: %v\n", err)
		return exitTool
	}
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
		pal.BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: true, InvariantMinimumDepth: 1, Tuning: checkerTuning})
	if err != nil {
		fmt.Fprintf(stderr, "test failed: %v\n", err)
		return exitTool
	}
	switch result.Outcome {
	case pal.SafetyNoCounterexample:
		fmt.Fprintln(stdout, "No example found.")
		return exitOK
	case pal.SafetyCounterexample:
		if err = writeTestViolationITF(outputPaths, prepared.Module, result.States); err != nil {
			fmt.Fprintf(stderr, "cannot write test violation trace: %v\n", err)
			return exitTool
		}
		fmt.Fprintf(stdout, "Assertion %s violated after %s.\n", assertion, action)
		if err = printDecodedStates(stdout, prepared.Module, result.States); err != nil {
			fmt.Fprintf(stderr, "cannot format test counterexample: %v\n", err)
			return exitTool
		}
		return exitCounterexample
	case pal.SafetyDeadlock:
		if err = writeTestViolationITF(outputPaths, prepared.Module, result.States); err != nil {
			fmt.Fprintf(stderr, "cannot write test deadlock trace: %v\n", err)
			return exitTool
		}
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

func writeTestViolationITF(paths commandOutputPaths, module *pal.Module, states []pal.DecodedState) error {
	if paths.runDir == "" && paths.customRunDir == "" {
		return nil
	}
	_, err := writeDecodedITFPaths(paths.implicitFiles("violation.itf.json"), module, states)
	return err
}

func runSimulate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("simulate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	backend := addCheckerBackendFlags(flags)
	length := flags.Int("length", 10, "maximum number of Next steps")
	maxRun := flags.Int("max-run", 100, "maximum number of independent symbolic simulation runs")
	maxErrors := flags.Int("max-error", 1, "produce at most this many distinct invariant errors")
	viewName := flags.String("view", "", "state view used to distinguish multiple errors")
	timeoutSMT := flags.Int("timeout-smt", 0, "limit each SMT query to this many seconds (0 is unlimited)")
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	outDir := flags.String("out-dir", "", "root directory for source-namespaced run output")
	runDir := flags.String("run-dir", "", "additional directory receiving run output directly")
	configPath := flags.String("config", "", "TLC configuration file")
	initName := flags.String("init", "", "initial-state operator")
	nextName := flags.String("next", "", "next-state operator")
	constantInit := flags.String("cinit", "", "constant initializer operator")
	itfPath := flags.String("itf", "", "write the simulated trace as ITF JSON")
	outputTraces := flags.Bool("output-traces", false, "save an ITF example for each symbolic run")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var invariants, temporalProperties csvStringList
	var libraryPaths stringList
	checkerTuning := pal.CheckerTuning{}
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	flags.Var(&invariants, "inv", "state invariant operator (repeatable)")
	flags.Var(&temporalProperties, "temporal", "temporal property operator (repeatable or comma-separated)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	if selectedConfig != "" {
		if err = applicationConfig.validateForSimulate(); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if err = applyJSONModelDefaults(applicationConfig, visited, jsonModelConfigTargets{
			backend: backend, length: length, timeoutSMT: timeoutSMT, TLCConfig: configPath,
			constantInit: constantInit, init: initName, next: nextName, useRows: useRows,
			disablePolytypes: disablePolytypes, invariants: &invariants,
			temporalProperties: &temporalProperties, maxErrors: maxErrors, view: viewName,
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if applicationConfig.Checker.Tuning != nil {
			checkerTuning, err = pal.ParseCheckerTuning(*applicationConfig.Checker.Tuning)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return exitUsage
			}
			if !visited["max-run"] {
				*maxRun = checkerTuning.SimulationRuns()
			}
			if !visited["output-traces"] && checkerTuning.HasOutputTraces() {
				*outputTraces = checkerTuning.OutputTraces()
			}
		}
	}
	resolveOutputDirectoryOptions(applicationConfig, visited, outDir, runDir)
	if visited["init"] != visited["next"] {
		fmt.Fprintln(stderr, "-init and -next must be provided together")
		return exitUsage
	}
	if flags.NArg() > 1 || (flags.NArg() == 0 && applicationConfig.Source == nil) {
		fmt.Fprintln(stderr, "simulate requires one TLA+ file from the CLI or JSON configuration")
		return exitUsage
	}
	sourcePath := ""
	if flags.NArg() == 1 {
		sourcePath = flags.Arg(0)
	} else {
		var cleanup func()
		sourcePath, cleanup, err = applicationConfig.Source.materializeSpecification()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		defer cleanup()
	}
	if *length < 0 {
		fmt.Fprintln(stderr, "simulate length cannot be negative")
		return exitUsage
	}
	if *maxRun <= 0 {
		fmt.Fprintln(stderr, "simulate max-run must be positive")
		return exitUsage
	}
	if *maxErrors <= 0 {
		fmt.Fprintln(stderr, "simulate max-error must be positive")
		return exitUsage
	}
	if *maxErrors > 1 && *viewName == "" {
		fmt.Fprintf(stderr, "checker.max-error=%d requires checker.view\n", *maxErrors)
		return exitUsage
	}
	configuredMaxRun := false
	if applicationConfig.Checker.Tuning != nil {
		_, configuredMaxRun = (*applicationConfig.Checker.Tuning)["search.simulation.maxRun"]
	}
	if *itfPath != "" && !visited["max-run"] && !configuredMaxRun {
		// A singular output path denotes one run. Explicit multi-run requests
		// use numbered sibling paths below.
		*maxRun = 1
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
		ConstantInit: *constantInit, Invariants: append([]string(nil), invariants...),
		TemporalProperties: append([]string(nil), temporalProperties...), View: *viewName,
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
	outputPaths, err := prepareCommandOutputPaths(*outDir, *runDir, sourcePath, "simulate")
	if err != nil {
		fmt.Fprintf(stderr, "cannot configure simulation output: %v\n", err)
		return exitUsage
	}
	if err = outputPaths.writeRunFile("simulate", args); err != nil {
		fmt.Fprintf(stderr, "cannot write simulation run metadata: %v\n", err)
		return exitTool
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
	prepared, err := pal.PrepareSafetyTransitions(frontend.Native, pal.SafetyPipelineOptions{
		Specification: configuration, Sources: frontend.Sources,
	})
	if err != nil {
		fmt.Fprintf(stderr, "cannot prepare simulation: %v\n", err)
		return exitTool
	}
	report, err := pal.SimulateAndCheckWithOptions(ctx, prepared.Module, prepared.Predicates,
		pal.SimulationOptions{MaxSteps: *length, MaxRuns: *maxRun, MaxErrors: *maxErrors, Tuning: checkerTuning})
	if err != nil {
		fmt.Fprintf(stderr, "simulation failed: %v\n", err)
		return exitTool
	}
	if len(report.Counterexamples) != 0 {
		for index, counterexample := range report.Counterexamples {
			if *outputTraces {
				name := traceITFPath("", "violation", index+1, len(report.Counterexamples))
				written, writeErr := writeDecodedITFPaths(outputPaths.implicitFiles(name), prepared.Module, counterexample.States)
				if writeErr != nil {
					fmt.Fprintf(stderr, "cannot write ITF simulation counterexample: %v\n", writeErr)
					return exitTool
				}
				for _, outputPath := range written {
					fmt.Fprintf(stdout, "Wrote ITF trace to %s.\n", outputPath)
				}
			}
			kind := "Invariant"
			if counterexample.TraceInvariant {
				kind = "Trace invariant"
			} else if counterexample.ActionInvariant {
				kind = "Action invariant"
			}
			fmt.Fprintf(stdout, "Simulation counterexample %d: %s %s violated at depth %d.\n",
				index+1, kind, counterexample.Invariant, counterexample.Depth)
			if err = printDecodedStates(stdout, prepared.Module, counterexample.States); err != nil {
				fmt.Fprintf(stderr, "cannot format simulation counterexample: %v\n", err)
				return exitTool
			}
		}
		return exitCounterexample
	}
	results := report.Runs
	for run, result := range results {
		if result.Outcome == pal.SimulationUnknown {
			fmt.Fprintf(stderr, "solver returned unknown in simulation run %d\n", run+1)
			return exitTool
		}
		if *itfPath != "" || *outputTraces {
			name := traceITFPath(*itfPath, "example", run+1, len(results))
			outputFiles := []string{name}
			if *itfPath == "" {
				outputFiles = outputPaths.implicitFiles(name)
			}
			written, writeErr := writeDecodedITFPaths(outputFiles, prepared.Module, result.States)
			if writeErr != nil {
				fmt.Fprintf(stderr, "cannot write ITF simulation: %v\n", writeErr)
				return exitTool
			}
			for _, outputPath := range written {
				fmt.Fprintf(stdout, "Wrote ITF trace to %s.\n", outputPath)
			}
		}
		if len(results) > 1 {
			fmt.Fprintf(stdout, "Simulation run %d of %d: ", run+1, len(results))
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
	}
	return exitOK
}

func simulationITFPath(path string, run, total int) string {
	if total <= 1 {
		return path
	}
	extension := filepath.Ext(path)
	return strings.TrimSuffix(path, extension) + fmt.Sprintf("-run-%d", run) + extension
}

func traceITFPath(explicitPath, prefix string, index, total int) string {
	if explicitPath != "" {
		return simulationITFPath(explicitPath, index, total)
	}
	if total <= 1 {
		return prefix + ".itf.json"
	}
	return fmt.Sprintf("%s%d.itf.json", prefix, index)
}

func writeDecodedITF(path string, module *pal.Module, states []pal.DecodedState) error {
	data, err := pal.MarshalDecodedTraceITF(module, states, pal.ITFOptions{})
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writeDecodedITFPaths(paths []string, module *pal.Module, states []pal.DecodedState) ([]string, error) {
	written := make([]string, 0, len(paths))
	for _, path := range paths {
		if err := writeDecodedITF(path, module, states); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	backend := addCheckerBackendFlags(flags)
	length := flags.Int("length", 10, "maximum number of Next steps")
	maxErrors := flags.Int("max-error", 1, "produce at most this many distinct errors")
	viewName := flags.String("view", "", "state view used to distinguish multiple errors")
	timeoutSMT := flags.Int("timeout-smt", 0, "limit each SMT query to this many seconds (0 is unlimited)")
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	outDir := flags.String("out-dir", "", "root directory for source-namespaced run output")
	runDir := flags.String("run-dir", "", "additional directory receiving run output directly")
	configPath := flags.String("config", "", "TLC configuration file")
	initName := flags.String("init", "", "initial-state operator")
	nextName := flags.String("next", "", "next-state operator")
	constantInit := flags.String("cinit", "", "constant initializer operator")
	deadlockFlag := flags.String("deadlock", "", "check deadlocks: true or false (default true)")
	noDeadlock := flags.Bool("no-deadlock", false, "disable deadlock checking (Scala-compatible alias)")
	itfPath := flags.String("itf", "", "write a counterexample trace as ITF JSON")
	outputTraces := flags.Bool("output-traces", false, "save an ITF example symbolic run")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var invariants, temporalProperties csvStringList
	var libraryPaths stringList
	checkerTuning := pal.CheckerTuning{}
	flags.Var(&invariants, "inv", "state invariant operator (repeatable)")
	flags.Var(&temporalProperties, "temporal", "temporal property operator (repeatable or comma-separated)")
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	if selectedConfig != "" {
		if err = applicationConfig.validateForCheck(); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if err = applyJSONModelDefaults(applicationConfig, visited, jsonModelConfigTargets{
			backend: backend, length: length, timeoutSMT: timeoutSMT, TLCConfig: configPath,
			constantInit: constantInit, init: initName, next: nextName, useRows: useRows,
			disablePolytypes: disablePolytypes, invariants: &invariants,
			temporalProperties: &temporalProperties, maxErrors: maxErrors, view: viewName,
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if applicationConfig.Checker.Tuning != nil {
			checkerTuning, err = pal.ParseCheckerTuning(*applicationConfig.Checker.Tuning)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return exitUsage
			}
			if !visited["output-traces"] && checkerTuning.HasOutputTraces() {
				*outputTraces = checkerTuning.OutputTraces()
			}
		}
	}
	resolveOutputDirectoryOptions(applicationConfig, visited, outDir, runDir)
	if visited["init"] != visited["next"] {
		fmt.Fprintln(stderr, "-init and -next must be provided together")
		return exitUsage
	}
	if flags.NArg() > 1 || (flags.NArg() == 0 && applicationConfig.Source == nil) {
		fmt.Fprintln(stderr, "check requires one TLA+ file from the CLI or JSON configuration")
		return exitUsage
	}
	sourcePath := ""
	if flags.NArg() == 1 {
		sourcePath = flags.Arg(0)
	} else {
		var cleanup func()
		sourcePath, cleanup, err = applicationConfig.Source.materializeSpecification()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		defer cleanup()
	}
	if *length < 0 {
		fmt.Fprintln(stderr, "check length cannot be negative")
		return exitUsage
	}
	if *maxErrors <= 0 {
		fmt.Fprintln(stderr, "check max-error must be positive")
		return exitUsage
	}
	if *maxErrors > 1 && *viewName == "" {
		fmt.Fprintf(stderr, "checker.max-error=%d requires checker.view\n", *maxErrors)
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
		View:               *viewName,
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
	if !visited["deadlock"] && !visited["no-deadlock"] && applicationConfig.Checker.NoDeadlock != nil {
		checkDeadlocks = !*applicationConfig.Checker.NoDeadlock
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
	outputPaths, err := prepareCommandOutputPaths(*outDir, *runDir, sourcePath, "check")
	if err != nil {
		fmt.Fprintf(stderr, "cannot configure check output: %v\n", err)
		return exitUsage
	}
	if err = outputPaths.writeRunFile("check", args); err != nil {
		fmt.Fprintf(stderr, "cannot write check run metadata: %v\n", err)
		return exitTool
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
	checkerTuning = checkerTuning.WithOutputTraces(*outputTraces)
	result, err := pal.CheckBoundedSafetyWithOptions(ctx, prepared.Module, prepared.Predicates, pal.BoundedSafetyOptions{
		MaxSteps: *length, CheckDeadlocks: checkDeadlocks, Tuning: checkerTuning, MaxErrors: *maxErrors,
	})
	if err != nil {
		fmt.Fprintf(stderr, "bounded safety check failed: %v\n", err)
		return exitTool
	}
	traceStates := [][]pal.DecodedState(nil)
	tracePrefix := "example"
	if result.Outcome == pal.SafetyCounterexample {
		tracePrefix = "violation"
		for _, counterexample := range result.Counterexamples {
			traceStates = append(traceStates, counterexample.States)
		}
	} else if result.Outcome == pal.SafetyDeadlock {
		tracePrefix, traceStates = "violation", append(traceStates, result.States)
	} else if result.Outcome == pal.SafetyNoCounterexample && *outputTraces && len(result.ExampleStates) != 0 {
		traceStates = append(traceStates, result.ExampleStates)
	}
	if *itfPath != "" || *outputTraces {
		for index, states := range traceStates {
			name := traceITFPath(*itfPath, tracePrefix, index+1, len(traceStates))
			outputFiles := []string{name}
			if *itfPath == "" {
				outputFiles = outputPaths.implicitFiles(name)
			}
			written, writeErr := writeDecodedITFPaths(outputFiles, prepared.Module, states)
			if writeErr != nil {
				fmt.Fprintf(stderr, "cannot write ITF trace: %v\n", writeErr)
				return exitTool
			}
			for _, outputPath := range written {
				fmt.Fprintf(stdout, "Wrote ITF trace to %s.\n", outputPath)
			}
		}
	}
	switch result.Outcome {
	case pal.SafetyCounterexample:
		for index, counterexample := range result.Counterexamples {
			kind := "Invariant"
			if counterexample.TraceInvariant {
				kind = "Trace invariant"
			} else if counterexample.ActionInvariant {
				kind = "Action invariant"
			}
			if len(result.Counterexamples) > 1 {
				fmt.Fprintf(stdout, "Counterexample %d: ", index+1)
			}
			fmt.Fprintf(stdout, "%s %s violated at depth %d (Init$%04d", kind, counterexample.Invariant,
				counterexample.Depth, counterexample.InitTransition)
			for _, number := range counterexample.NextTransitions {
				fmt.Fprintf(stdout, ", Next$%04d", number)
			}
			fmt.Fprintln(stdout, ").")
			if formatErr := printDecodedStates(stdout, prepared.Module, counterexample.States); formatErr != nil {
				fmt.Fprintf(stderr, "cannot format counterexample state: %v\n", formatErr)
				return exitTool
			}
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
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	includeSource := flags.Bool("source", false, "include source positions in JSON IR")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var libraryPaths stringList
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	if selectedConfig != "" {
		if err = applicationConfig.validateForFrontend("typecheck"); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if err = applyJSONFrontendDefaults(applicationConfig, visited, output, features, useRows, disablePolytypes); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
	}
	if flags.NArg() > 1 || (flags.NArg() == 0 && applicationConfig.Source == nil) {
		fmt.Fprintln(stderr, "typecheck requires one TLA+ file from the CLI or JSON configuration")
		return exitUsage
	}
	sourcePath := ""
	if flags.NArg() == 1 {
		sourcePath = flags.Arg(0)
	} else {
		var cleanup func()
		sourcePath, cleanup, err = applicationConfig.Source.materializeSpecification()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		defer cleanup()
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

	result := pal.ParseAndTypeCheckFile(ctx, sourcePath, pal.FrontendOptions{
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
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	includeSource := flags.Bool("source", false, "include source positions in JSON IR")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	var libraryPaths stringList
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	if selectedConfig != "" {
		if err = applicationConfig.validateForFrontend("parse"); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if err = applyJSONFrontendDefaults(applicationConfig, visited, output, features, nil, nil); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
	}
	if flags.NArg() > 1 || (flags.NArg() == 0 && applicationConfig.Source == nil) {
		fmt.Fprintln(stderr, "parse requires one TLA+ file from the CLI or JSON configuration")
		return exitUsage
	}
	sourcePath := ""
	if flags.NArg() == 1 {
		sourcePath = flags.Arg(0)
	} else {
		var cleanup func()
		sourcePath, cleanup, err = applicationConfig.Source.materializeSpecification()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		defer cleanup()
	}
	if !validModuleOutput(*output) {
		fmt.Fprintf(stderr, "invalid output %q (want summary, json, or a .tla/.json file)\n", *output)
		return exitUsage
	}
	if _, err := resolveRowsFeature(false, *features); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}

	result := pal.ParseFile(ctx, sourcePath, pal.FrontendOptions{
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
		source, err := os.ReadFile(sourcePath)
		if err != nil {
			fmt.Fprintf(stderr, "cannot recover source annotations: %v\n", err)
			return exitFrontend
		}
		annotations := pal.ExtractTypeAnnotations(sourcePath, string(source), result.Native, result.Sources)
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
	fmt.Fprintln(w, "  server        run the JSON-RPC exploration server")
	fmt.Fprintln(w, "  simulate FILE generate one bounded behavior")
	fmt.Fprintln(w, "  test FILE BEFORE ACTION ASSERTION run a one-step test")
	fmt.Fprintln(w, "  tracee SOURCE TRACE EXPRESSIONS evaluate operators over an ITF trace")
	fmt.Fprintln(w, "  transpile FILE transpile ReTLA to VMT constraints")
	fmt.Fprintln(w, "  typecheck FILE parse and type-check annotated TLA+")
	fmt.Fprintln(w, "  version       print build version")
}
