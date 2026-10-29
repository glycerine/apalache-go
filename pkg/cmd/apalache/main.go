package main

import (
	"bufio"
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
	serverType := flags.String("server-type", "explorer", "server type: 'checker' (shai-grpc) or 'explorer' (json-rpc)")
	documentFlagDefaults(flags)
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
	commonOutput := addCommonOutputFlags(flags)
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
	documentFlagDefaults(flags)
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
	commonOptions, err := commonOutput.resolve(applicationConfig, visited)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
	effectiveInit, effectiveNext := *initName, *nextName
	if effectiveInit == "" && configuration.TLCConfig == nil {
		effectiveInit, effectiveNext = "Init", "Next"
	}
	debugOverlay := transpileDebugApplicationOverlay(
		*outputPath, *configPath, *constantInit, effectiveInit, effectiveNext,
		invariants, temporalProperties, rows, !*disablePolytypes)
	if err = writeDebugApplicationConfig(outputPaths, applicationConfig, "transpile", selectedConfig, sourcePath, *outDir, *runDir, commonOptions, debugOverlay); err != nil {
		fmt.Fprintf(stderr, "cannot write transpile debug configuration: %v\n", err)
		return exitTool
	}
	frontend := pal.ParseAndTypeCheckFile(ctx, sourcePath, pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, NoRows: *features == "no-rows", DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range frontend.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !frontend.OK() {
		return exitFrontend
	}
	if commonOptions.writeIntermediate {
		if err = writeFrontendIntermediates(outputPaths, frontend); err != nil {
			fmt.Fprintf(stderr, "cannot write transpile intermediate output: %v\n", err)
			return exitTool
		}
	}
	prepared, err := pal.PrepareSafetyTransitions(frontend.Native, pal.SafetyPipelineOptions{
		Specification: configuration,
		Sources:       frontend.Sources,
		OnStage:       safetyIntermediateObserver(commonOptions.writeIntermediate, outputPaths, frontend.Extends, frontend.Sources),
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
	commonOutput := addCommonOutputFlags(flags)
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	outDir := flags.String("out-dir", "", "root directory for source-namespaced run output")
	runDir := flags.String("run-dir", "", "additional directory receiving run output directly")
	tracePath := flags.String("trace", "", "input ITF trace")
	itfPath := flags.String("itf", "", "write evaluated ITF trace to this file (default stdout)")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var expressions csvStringList
	var libraryPaths stringList
	flags.Var(&expressions, "expressions", "nullary operators to evaluate (repeatable or comma-separated)")
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	documentFlagDefaults(flags)
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
	commonOptions, err := commonOutput.resolve(applicationConfig, visited)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
	resolveOutputDirectoryOptions(applicationConfig, visited, outDir, runDir)
	if sourcePath == "" || *tracePath == "" || len(expressions) == 0 {
		fmt.Fprintln(stderr, "tracee requires a trace and at least one expression operator")
		return exitUsage
	}
	rows, featureErr := resolveRowsFeature(*useRows, *features)
	if featureErr != nil {
		fmt.Fprintln(stderr, featureErr)
		return exitUsage
	}
	outputPaths, err := prepareCommandOutputPaths(*outDir, *runDir, sourcePath, "tracee")
	if err != nil {
		fmt.Fprintf(stderr, "cannot configure tracee output: %v\n", err)
		return exitUsage
	}
	if err = outputPaths.writeRunFile("tracee", args); err != nil {
		fmt.Fprintf(stderr, "cannot write tracee run metadata: %v\n", err)
		return exitTool
	}
	debugOverlay := traceeDebugApplicationOverlay(*tracePath, expressions, rows, !*disablePolytypes)
	if err = writeDebugApplicationConfig(outputPaths, applicationConfig, "tracee", selectedConfig, sourcePath, *outDir, *runDir, commonOptions, debugOverlay); err != nil {
		fmt.Fprintf(stderr, "cannot write tracee debug configuration: %v\n", err)
		return exitTool
	}
	frontend := pal.ParseAndTypeCheckFile(ctx, sourcePath, pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, NoRows: *features == "no-rows", DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range frontend.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !frontend.OK() {
		return exitFrontend
	}
	if commonOptions.writeIntermediate {
		if err = writeFrontendIntermediates(outputPaths, frontend); err != nil {
			fmt.Fprintf(stderr, "cannot write tracee intermediate output: %v\n", err)
			return exitTool
		}
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
	if *itfPath == "" && outputPaths.runDir == "" && outputPaths.customRunDir == "" {
		if _, err = stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "cannot write evaluated trace: %v\n", err)
			return exitTool
		}
	} else {
		outputFiles := []string{*itfPath}
		if *itfPath == "" {
			outputFiles = outputPaths.implicitFiles("example.itf.json")
		}
		for _, outputPath := range outputFiles {
			if err = os.WriteFile(outputPath, data, 0o644); err != nil {
				fmt.Fprintf(stderr, "cannot write evaluated trace: %v\n", err)
				return exitTool
			}
			fmt.Fprintf(stdout, "Wrote evaluated ITF trace to %s.\n", outputPath)
		}
	}
	return exitOK
}

func runConfig(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("config", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var enableStats optionalBoolFlag
	flags.Var(&enableStats, "enable-stats", "enable shared TLA+ usage statistics: true or false (default unchanged)")
	documentFlagDefaults(flags)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "config does not accept positional arguments")
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
	if enableStats.set {
		contents := "NO_STATISTICS"
		if enableStats.value {
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

type optionalBoolFlag struct {
	set   bool
	value bool
}

func (value *optionalBoolFlag) String() string {
	if !value.set {
		return ""
	}
	return strconv.FormatBool(value.value)
}

func (value *optionalBoolFlag) Set(text string) error {
	switch strings.ToLower(text) {
	case "", "true", "yes", "1":
		value.set, value.value = true, true
	case "false", "no", "0":
		value.set, value.value = true, false
	default:
		return fmt.Errorf("must be true or false")
	}
	return nil
}

func (*optionalBoolFlag) IsBoolFlag() bool { return true }

func runTest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(stderr)
	commonOutput := addCommonOutputFlags(flags)
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
	documentFlagDefaults(flags)
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
	commonOptions, err := commonOutput.resolve(applicationConfig, visited)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
	seed := uint32(time.Now().UnixMilli() & 0x7fffffff)
	if seed == 0 {
		seed = 1
	}
	checkerTuning = checkerTuning.WithSMTRandomSeed(seed)
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
	rows, featureErr := resolveRowsFeature(*useRows, *features)
	if featureErr != nil {
		fmt.Fprintln(stderr, featureErr)
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
	debugTuning := make(map[string]string)
	if applicationConfig.Checker.Tuning != nil {
		for key, value := range *applicationConfig.Checker.Tuning {
			debugTuning[key] = value
		}
	}
	debugTuning["smt.randomSeed"] = strconv.FormatUint(uint64(seed), 10)
	debugBackend := &checkerBackendOptions{algorithm: "incremental", encoding: "oopsla19", solver: "z3"}
	debugOverlay := checkDebugApplicationOverlay(
		debugBackend, debugTuning, "", *constantInit, before, action, []string{assertion}, nil,
		1, 1, 0, true, true, !*disablePolytypes, rows, "")
	if err = writeDebugApplicationConfig(outputPaths, applicationConfig, "test", selectedConfig, path, *outDir, *runDir, commonOptions, debugOverlay); err != nil {
		fmt.Fprintf(stderr, "cannot write test debug configuration: %v\n", err)
		return exitTool
	}
	frontend := pal.ParseAndTypeCheckFile(ctx, path, pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, NoRows: *features == "no-rows", DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range frontend.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !frontend.OK() {
		return exitFrontend
	}
	if commonOptions.writeIntermediate {
		if err = writeFrontendIntermediates(outputPaths, frontend); err != nil {
			fmt.Fprintf(stderr, "cannot write test intermediate output: %v\n", err)
			return exitTool
		}
	}
	changes := pal.NewChangeListener()
	metricStats := newMetricProfiler(commonOptions.smtProfile, outputPaths, frontend.Sources, changes)
	var tracker pal.TransformationTracker
	if metricStats != nil {
		tracker = pal.NewTransformationTrackerProvider(changes).Get()
	}
	prepared, err := pal.PrepareSafetyTransitions(frontend.Native, pal.SafetyPipelineOptions{
		Specification: pal.SpecificationConfiguration{
			Behavior: pal.InitNextSpec{Init: before, Next: action}, ConstantInit: *constantInit,
			Invariants: []string{assertion},
		},
		Sources: frontend.Sources, Tracker: tracker,
		OnStage: safetyIntermediateObserver(commonOptions.writeIntermediate, outputPaths, frontend.Extends, frontend.Sources),
	})
	if err != nil {
		fmt.Fprintf(stderr, "cannot prepare test: %v\n", err)
		return exitTool
	}
	ruleStats := newRuleProfiler(commonOptions.profiling)
	result, err := pal.CheckBoundedSafetyWithOptions(ctx, prepared.Module, prepared.Predicates,
		pal.BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: true, InvariantMinimumDepth: 1, Tuning: checkerTuning,
			RuleStats: ruleStats, RewriteStats: metricStats})
	if profileErr := writeRuleProfile(outputPaths, ruleStats); profileErr != nil {
		fmt.Fprintf(stderr, "cannot write test rule profile: %v\n", profileErr)
		return exitTool
	}
	if profileErr := writeMetricProfile(outputPaths, metricStats); profileErr != nil {
		fmt.Fprintf(stderr, "cannot write test SMT profile: %v\n", profileErr)
		return exitTool
	}
	if err != nil {
		fmt.Fprintf(stderr, "test failed: %v\n", err)
		return exitTool
	}
	switch result.Outcome {
	case pal.SafetyNoCounterexample:
		fmt.Fprintln(stdout, "No example found.")
		return exitOK
	case pal.SafetyCounterexample:
		if err = writeTestViolationTraces(outputPaths, prepared.Module, result.States, assertion); err != nil {
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
		if err = writeTestViolationTraces(outputPaths, prepared.Module, result.States, ""); err != nil {
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

func writeTestViolationTraces(paths commandOutputPaths, module *pal.Module, states []pal.DecodedState, invariant string) error {
	if paths.runDir == "" && paths.customRunDir == "" {
		return nil
	}
	_, err := writeDecodedTraceFormatPaths(paths.implicitFiles("violation.itf.json"), module, states,
		traceViolationExpression(module, invariant))
	return err
}

func runSimulate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("simulate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	commonOutput := addCommonOutputFlags(flags)
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
	documentFlagDefaults(flags)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	applyCheckerBackendEnvironment(backend, visited)
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	commonOptions, err := commonOutput.resolve(applicationConfig, visited)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
	effectiveInit, effectiveNext := *initName, *nextName
	if effectiveInit == "" && configuration.TLCConfig == nil {
		effectiveInit, effectiveNext = "Init", "Next"
	}
	debugTuning := make(map[string]string)
	if applicationConfig.Checker.Tuning != nil {
		for key, value := range *applicationConfig.Checker.Tuning {
			debugTuning[key] = value
		}
	}
	debugTuning["search.simulation.maxRun"] = strconv.Itoa(*maxRun)
	debugTuning["search.outputTraces"] = strconv.FormatBool(*outputTraces)
	checkDeadlocks := applicationConfig.Checker.NoDeadlock == nil || !*applicationConfig.Checker.NoDeadlock
	debugOverlay := checkDebugApplicationOverlay(
		backend, debugTuning, *configPath, *constantInit, effectiveInit, effectiveNext,
		invariants, temporalProperties, *length, *maxErrors, *timeoutSMT,
		true, checkDeadlocks, !*disablePolytypes, rows, *viewName)
	if err = writeDebugApplicationConfig(outputPaths, applicationConfig, "simulate", selectedConfig, sourcePath, *outDir, *runDir, commonOptions, debugOverlay); err != nil {
		fmt.Fprintf(stderr, "cannot write simulate debug configuration: %v\n", err)
		return exitTool
	}

	frontend := pal.ParseAndTypeCheckFile(ctx, sourcePath, pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, NoRows: *features == "no-rows", DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range frontend.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !frontend.OK() {
		return exitFrontend
	}
	if commonOptions.writeIntermediate {
		if err = writeFrontendIntermediates(outputPaths, frontend); err != nil {
			fmt.Fprintf(stderr, "cannot write simulate intermediate output: %v\n", err)
			return exitTool
		}
	}
	changes := pal.NewChangeListener()
	metricStats := newMetricProfiler(commonOptions.smtProfile, outputPaths, frontend.Sources, changes)
	var tracker pal.TransformationTracker
	if metricStats != nil {
		tracker = pal.NewTransformationTrackerProvider(changes).Get()
	}
	prepared, err := pal.PrepareSafetyTransitions(frontend.Native, pal.SafetyPipelineOptions{
		Specification: configuration, Sources: frontend.Sources, Tracker: tracker,
		OnStage: safetyIntermediateObserver(commonOptions.writeIntermediate, outputPaths, frontend.Extends, frontend.Sources),
	})
	if err != nil {
		fmt.Fprintf(stderr, "cannot prepare simulation: %v\n", err)
		return exitTool
	}
	ruleStats := newRuleProfiler(commonOptions.profiling)
	report, err := pal.SimulateAndCheckWithOptions(ctx, prepared.Module, prepared.Predicates,
		pal.SimulationOptions{MaxSteps: *length, MaxRuns: *maxRun, MaxErrors: *maxErrors, Tuning: checkerTuning,
			SMTEncoding: pal.SMTEncoding(backend.encoding), RuleStats: ruleStats, RewriteStats: metricStats})
	if profileErr := writeRuleProfile(outputPaths, ruleStats); profileErr != nil {
		fmt.Fprintf(stderr, "cannot write simulation rule profile: %v\n", profileErr)
		return exitTool
	}
	if profileErr := writeMetricProfile(outputPaths, metricStats); profileErr != nil {
		fmt.Fprintf(stderr, "cannot write simulation SMT profile: %v\n", profileErr)
		return exitTool
	}
	if err != nil {
		fmt.Fprintf(stderr, "simulation failed: %v\n", err)
		return exitTool
	}
	if len(report.Counterexamples) != 0 {
		for index, counterexample := range report.Counterexamples {
			if *outputTraces {
				name := traceITFPath("", "violation", index+1, len(report.Counterexamples))
				written, writeErr := writeDecodedTraceFormatPaths(outputPaths.implicitFiles(name), prepared.Module,
					counterexample.States, traceViolationExpression(prepared.Module, counterexample.Invariant))
				if writeErr != nil {
					fmt.Fprintf(stderr, "cannot write simulation counterexample traces: %v\n", writeErr)
					return exitTool
				}
				for _, outputPath := range written {
					fmt.Fprintf(stdout, "Wrote trace to %s.\n", outputPath)
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
			var written []string
			var writeErr error
			if *itfPath == "" {
				written, writeErr = writeDecodedTraceFormatPaths(outputFiles, prepared.Module, result.States, nil)
			} else {
				written, writeErr = writeDecodedITFPaths(outputFiles, prepared.Module, result.States)
			}
			if writeErr != nil {
				fmt.Fprintf(stderr, "cannot write simulation trace: %v\n", writeErr)
				return exitTool
			}
			for _, outputPath := range written {
				fmt.Fprintf(stdout, "Wrote trace to %s.\n", outputPath)
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

func writeDecodedTraceFormats(
	itfPath string,
	module *pal.Module,
	states []pal.DecodedState,
	violation pal.Expr,
) ([]string, error) {
	prefix := strings.TrimSuffix(itfPath, ".itf.json")
	if prefix == itfPath {
		return nil, fmt.Errorf("implicit trace path %s does not end in .itf.json", itfPath)
	}
	tla, err := pal.FormatDecodedTraceTLA(module, states, violation)
	if err != nil {
		return nil, err
	}
	tlc, err := pal.FormatDecodedTraceTLC(module, states)
	if err != nil {
		return nil, err
	}
	legacy, err := pal.MarshalDecodedTraceApalacheJSON(module, states, violation)
	if err != nil {
		return nil, err
	}
	tlcPath := filepath.Join(filepath.Dir(prefix), "MC"+filepath.Base(prefix)+".out")
	files := []struct {
		path string
		data []byte
	}{
		{prefix + ".tla", []byte(tla)},
		{tlcPath, []byte(tlc)},
		{prefix + ".json", append(legacy, '\n')},
	}
	written := make([]string, 0, 4)
	for _, file := range files {
		if err = os.WriteFile(file.path, file.data, 0o644); err != nil {
			return written, err
		}
		written = append(written, file.path)
	}
	if err = writeDecodedITF(itfPath, module, states); err != nil {
		return written, err
	}
	return append(written, itfPath), nil
}

func writeDecodedTraceFormatPaths(
	itfPaths []string,
	module *pal.Module,
	states []pal.DecodedState,
	violation pal.Expr,
) ([]string, error) {
	written := make([]string, 0, len(itfPaths)*4)
	for _, path := range itfPaths {
		files, err := writeDecodedTraceFormats(path, module, states, violation)
		if err != nil {
			return written, err
		}
		written = append(written, files...)
	}
	return written, nil
}

func traceViolationExpression(module *pal.Module, name string) pal.Expr {
	if module != nil && name != "" {
		for _, declaration := range module.OperDeclarations() {
			if declaration.Name() == name {
				return pal.MustOperExpr(pal.Op("NOT"), pal.Typed(pal.BoolT1), declaration.Body())
			}
		}
	}
	return pal.NewValueExpr(pal.BoolValue(true), pal.Typed(pal.BoolT1))
}

func parseTuningOptions(text string) (map[string]string, error) {
	result := make(map[string]string)
	for _, entry := range strings.Split(text, ":") {
		parts := strings.Split(entry, "=")
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || parts[1] == "" {
			return nil, fmt.Errorf("expected key=value in --tuning-options=%s", text)
		}
		result[strings.TrimSpace(parts[0])] = parts[1]
	}
	return result, nil
}

func loadTuningProperties(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read tuning properties file %s: %w", path, err)
	}
	defer file.Close()
	result := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") || strings.HasPrefix(text, "!") {
			continue
		}
		separator := strings.IndexAny(text, "=:")
		if separator < 0 || strings.TrimSpace(text[:separator]) == "" || strings.TrimSpace(text[separator+1:]) == "" {
			return nil, fmt.Errorf("invalid tuning property at %s:%d: expected key=value", path, line)
		}
		result[strings.TrimSpace(text[:separator])] = strings.TrimSpace(text[separator+1:])
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("cannot read tuning properties file %s: %w", path, err)
	}
	return result, nil
}

func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	commonOutput := addCommonOutputFlags(flags)
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
	var discardDisabledFlag optionalBoolFlag
	flags.Var(&discardDisabledFlag, "discard-disabled", "eagerly check and discard disabled transitions")
	tuningOptions := flags.String("tuning-options", "", "checker tuning as colon-separated key=value pairs")
	tuningOptionsFile := flags.String("tuning-options-file", "", "properties file containing checker tuning")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var invariants, temporalProperties csvStringList
	var libraryPaths stringList
	checkerTuning := pal.CheckerTuning{}
	tuningValues := make(map[string]string)
	discardDisabled := true
	flags.Var(&invariants, "inv", "state invariant operator (repeatable)")
	flags.Var(&temporalProperties, "temporal", "temporal property operator (repeatable or comma-separated)")
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	documentFlagDefaults(flags)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	applyCheckerBackendEnvironment(backend, visited)
	applicationConfig, selectedConfig, err := loadCommandConfig(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot load Apalache configuration: %v\n", err)
		return exitUsage
	}
	commonOptions, err := commonOutput.resolve(applicationConfig, visited)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
			discardDisabled: &discardDisabled,
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if applicationConfig.Checker.Tuning != nil {
			for key, value := range *applicationConfig.Checker.Tuning {
				tuningValues[key] = value
			}
		}
	}
	if discardDisabledFlag.set {
		discardDisabled = discardDisabledFlag.value
	}
	if *tuningOptionsFile != "" {
		fileValues, parseErr := loadTuningProperties(*tuningOptionsFile)
		if parseErr != nil {
			fmt.Fprintln(stderr, parseErr)
			return exitUsage
		}
		for key, value := range fileValues {
			tuningValues[key] = value
		}
	}
	if *tuningOptions != "" {
		inlineValues, parseErr := parseTuningOptions(*tuningOptions)
		if parseErr != nil {
			fmt.Fprintln(stderr, parseErr)
			return exitUsage
		}
		for key, value := range inlineValues {
			tuningValues[key] = value
		}
	}
	checkerTuning, err = pal.ParseCheckerTuning(tuningValues)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if !visited["output-traces"] && checkerTuning.HasOutputTraces() {
		*outputTraces = checkerTuning.OutputTraces()
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
	effectiveInit, effectiveNext := *initName, *nextName
	if effectiveInit == "" && configuration.TLCConfig == nil {
		effectiveInit, effectiveNext = "Init", "Next"
	}
	debugTuning := make(map[string]string, len(tuningValues)+1)
	for key, value := range tuningValues {
		debugTuning[key] = value
	}
	debugTuning["search.outputTraces"] = strconv.FormatBool(*outputTraces)
	debugOverlay := checkDebugApplicationOverlay(
		backend, debugTuning, *configPath, *constantInit, effectiveInit, effectiveNext,
		invariants, temporalProperties, *length, *maxErrors, *timeoutSMT,
		discardDisabled, checkDeadlocks, !*disablePolytypes, rows, *viewName)
	if err = writeDebugApplicationConfig(outputPaths, applicationConfig, "check", selectedConfig, sourcePath, *outDir, *runDir, commonOptions, debugOverlay); err != nil {
		fmt.Fprintf(stderr, "cannot write check debug configuration: %v\n", err)
		return exitTool
	}

	frontend := pal.ParseAndTypeCheckFile(ctx, sourcePath, pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, NoRows: *features == "no-rows", DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range frontend.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !frontend.OK() {
		return exitFrontend
	}
	if commonOptions.writeIntermediate {
		if err = writeFrontendIntermediates(outputPaths, frontend); err != nil {
			fmt.Fprintf(stderr, "cannot write check intermediate output: %v\n", err)
			return exitTool
		}
	}
	changes := pal.NewChangeListener()
	metricStats := newMetricProfiler(commonOptions.smtProfile, outputPaths, frontend.Sources, changes)
	var tracker pal.TransformationTracker
	if metricStats != nil {
		tracker = pal.NewTransformationTrackerProvider(changes).Get()
	}
	prepared, err := pal.PrepareSafetyTransitions(frontend.Native, pal.SafetyPipelineOptions{
		Specification: configuration,
		Sources:       frontend.Sources,
		Tracker:       tracker,
		OnStage:       safetyIntermediateObserver(commonOptions.writeIntermediate, outputPaths, frontend.Extends, frontend.Sources),
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
	ruleStats := newRuleProfiler(commonOptions.profiling)
	result, err := pal.CheckBoundedSafetyWithOptions(ctx, prepared.Module, prepared.Predicates, pal.BoundedSafetyOptions{
		MaxSteps: *length, CheckDeadlocks: checkDeadlocks, Tuning: checkerTuning, MaxErrors: *maxErrors,
		DiscardDisabled: &discardDisabled, SMTEncoding: pal.SMTEncoding(backend.encoding), RuleStats: ruleStats,
		RewriteStats: metricStats,
	})
	if profileErr := writeRuleProfile(outputPaths, ruleStats); profileErr != nil {
		fmt.Fprintf(stderr, "cannot write check rule profile: %v\n", profileErr)
		return exitTool
	}
	if profileErr := writeMetricProfile(outputPaths, metricStats); profileErr != nil {
		fmt.Fprintf(stderr, "cannot write check SMT profile: %v\n", profileErr)
		return exitTool
	}
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
			var violation pal.Expr
			if result.Outcome == pal.SafetyCounterexample && index < len(result.Counterexamples) {
				violation = traceViolationExpression(prepared.Module, result.Counterexamples[index].Invariant)
			}
			var written []string
			var writeErr error
			if *itfPath == "" {
				written, writeErr = writeDecodedTraceFormatPaths(outputFiles, prepared.Module, states, violation)
			} else {
				written, writeErr = writeDecodedITFPaths(outputFiles, prepared.Module, states)
			}
			if writeErr != nil {
				fmt.Fprintf(stderr, "cannot write trace: %v\n", writeErr)
				return exitTool
			}
			for _, outputPath := range written {
				fmt.Fprintf(stdout, "Wrote trace to %s.\n", outputPath)
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
	commonOutput := addCommonOutputFlags(flags)
	output := flags.String("output", "summary", "output format (summary/json) or .tla/.json file")
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	outDir := flags.String("out-dir", "", "root directory for source-namespaced run output")
	runDir := flags.String("run-dir", "", "additional directory receiving run output directly")
	includeSource := flags.Bool("source", false, "include source positions in JSON IR")
	useRows := flags.Bool("rows", false, "enable row types")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	disablePolytypes := flags.Bool("no-polytypes", false, "disable let-polymorphism")
	var libraryPaths stringList
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	documentFlagDefaults(flags)
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
	commonOptions, err := commonOutput.resolve(applicationConfig, visited)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
	resolveOutputDirectoryOptions(applicationConfig, visited, outDir, runDir)
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
	outputPaths, err := prepareCommandOutputPaths(*outDir, *runDir, sourcePath, "typecheck")
	if err != nil {
		fmt.Fprintf(stderr, "cannot configure typecheck output: %v\n", err)
		return exitUsage
	}
	if err = outputPaths.writeRunFile("typecheck", args); err != nil {
		fmt.Fprintf(stderr, "cannot write typecheck run metadata: %v\n", err)
		return exitTool
	}
	debugOverlay := frontendDebugApplicationOverlay(*output, rows, !*disablePolytypes)
	if err = writeDebugApplicationConfig(outputPaths, applicationConfig, "typecheck", selectedConfig, sourcePath, *outDir, *runDir, commonOptions, debugOverlay); err != nil {
		fmt.Fprintf(stderr, "cannot write typecheck debug configuration: %v\n", err)
		return exitTool
	}
	result := pal.ParseAndTypeCheckFile(ctx, sourcePath, pal.FrontendOptions{
		LibraryPaths: libraryPaths,
	}, pal.TypeCheckOptions{UseRows: rows, NoRows: *features == "no-rows", DisablePolytypes: *disablePolytypes})
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if !result.OK() {
		return exitFrontend
	}
	if commonOptions.writeIntermediate {
		if err = writeFrontendIntermediates(outputPaths, result); err != nil {
			fmt.Fprintf(stderr, "cannot write typecheck intermediate output: %v\n", err)
			return exitTool
		}
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
	commonOutput := addCommonOutputFlags(flags)
	output := flags.String("output", "summary", "output format (summary/json) or .tla/.json file")
	configFile := flags.String("config-file", "", "Apalache JSON configuration file")
	outDir := flags.String("out-dir", "", "root directory for source-namespaced run output")
	runDir := flags.String("run-dir", "", "additional directory receiving run output directly")
	includeSource := flags.Bool("source", false, "include source positions in JSON IR")
	features := flags.String("features", "", "typechecker features: rows or no-rows")
	var libraryPaths stringList
	flags.Var(&libraryPaths, "I", "add a TLA+ module search directory (repeatable)")
	documentFlagDefaults(flags)
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
	commonOptions, err := commonOutput.resolve(applicationConfig, visited)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
	resolveOutputDirectoryOptions(applicationConfig, visited, outDir, runDir)
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
	rows, featureErr := resolveRowsFeature(false, *features)
	if featureErr != nil {
		fmt.Fprintln(stderr, featureErr)
		return exitUsage
	}
	outputPaths, err := prepareCommandOutputPaths(*outDir, *runDir, sourcePath, "parse")
	if err != nil {
		fmt.Fprintf(stderr, "cannot configure parse output: %v\n", err)
		return exitUsage
	}
	if err = outputPaths.writeRunFile("parse", args); err != nil {
		fmt.Fprintf(stderr, "cannot write parse run metadata: %v\n", err)
		return exitTool
	}
	debugOverlay := frontendDebugApplicationOverlay(*output, rows, true)
	if err = writeDebugApplicationConfig(outputPaths, applicationConfig, "parse", selectedConfig, sourcePath, *outDir, *runDir, commonOptions, debugOverlay); err != nil {
		fmt.Fprintf(stderr, "cannot write parse debug configuration: %v\n", err)
		return exitTool
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
	if commonOptions.writeIntermediate {
		if err = writeFrontendIntermediates(outputPaths, result); err != nil {
			fmt.Fprintf(stderr, "cannot write parsed intermediate output: %v\n", err)
			return exitTool
		}
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
	flags.StringVar(&options.algorithm, "algo", "incremental", "checking algorithm: incremental, offline, or remote (used by explorer)")
	flags.StringVar(&options.encoding, "smt-encoding", "oopsla19", "SMT encoding: oopsla19, arrays (experimental), or funArrays (experimental)")
	flags.StringVar(&options.solver, "smt-solver", "z3", "SMT solver backend: z3 via g3 or cvc5 (experimental)")
	return options
}

func documentFlagDefaults(flags *flag.FlagSet) {
	if flags == nil {
		return
	}
	flags.VisitAll(func(option *flag.Flag) {
		if strings.Contains(strings.ToLower(option.Usage), "default:") {
			return
		}
		value := option.DefValue
		if value == "" {
			value = "<empty>"
		}
		option.Usage += " (default: " + value + ")"
	})
}

func applyCheckerBackendEnvironment(options *checkerBackendOptions, visited map[string]bool) {
	if options == nil {
		return
	}
	if !visited["smt-encoding"] {
		if value, found := os.LookupEnv("SMT_ENCODING"); found {
			options.encoding = value
			visited["smt-encoding"] = true
		}
	}
	if !visited["smt-solver"] {
		if value, found := os.LookupEnv("SMT_SOLVER"); found {
			options.solver = value
			visited["smt-solver"] = true
		}
	}
}

func validateCheckerBackendOptions(options *checkerBackendOptions) error {
	if options == nil {
		return fmt.Errorf("checker backend options are missing")
	}
	if algorithm := strings.ToLower(options.algorithm); algorithm != "incremental" {
		return fmt.Errorf("unsupported checker algorithm %q (Go port currently supports incremental)", options.algorithm)
	}
	if _, err := pal.ParseSMTEncoding(options.encoding); err != nil {
		return fmt.Errorf("unsupported SMT encoding %q (want oopsla19, arrays, or funArrays)", options.encoding)
	}
	if solver := strings.ToLower(options.solver); solver != "z3" {
		return fmt.Errorf("unsupported SMT solver %q (Go port uses g3 v0.0.8 as its Z3 backend)", options.solver)
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
