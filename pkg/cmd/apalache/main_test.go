package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	pal "github.com/glycerine/apalache-go"
	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestConcurrentParseRunsKeepIntermediateOutputIsolated(t *testing.T) {
	root := t.TempDir()
	writeSpec := func(module, declaration string) string {
		path := filepath.Join(root, module+".tla")
		if err := os.WriteFile(path, []byte("---- MODULE "+module+" ----\n"+declaration+"\n====\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	firstSource := writeSpec("First", "FirstValue == 1")
	secondSource := writeSpec("Second", "SecondValue == 2")
	type result struct {
		code   int
		stderr string
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var group sync.WaitGroup
	runConcurrent := func(source, outDir string) {
		defer group.Done()
		<-start
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"parse", "--out-dir=" + outDir, "--write-intermediate=true", source,
		}, &stdout, &stderr)
		results <- result{code: code, stderr: stderr.String()}
	}
	firstOut, secondOut := filepath.Join(root, "first-out"), filepath.Join(root, "second-out")
	group.Add(2)
	go runConcurrent(firstSource, firstOut)
	go runConcurrent(secondSource, secondOut)
	close(start)
	group.Wait()
	close(results)
	for result := range results {
		if result.code != exitOK {
			t.Fatalf("concurrent parse = %d, stderr = %q", result.code, result.stderr)
		}
	}
	readIntermediate := func(outDir, sourceName string) string {
		matches, err := filepath.Glob(filepath.Join(outDir, sourceName, "*", "intermediate", "00_OutSanyParser.tla"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("intermediate under %s = %v, %v", outDir, matches, err)
		}
		contents, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatal(err)
		}
		return string(contents)
	}
	firstText := readIntermediate(firstOut, "First.tla")
	secondText := readIntermediate(secondOut, "Second.tla")
	if !strings.Contains(firstText, "FirstValue") || strings.Contains(firstText, "SecondValue") ||
		!strings.Contains(secondText, "SecondValue") || strings.Contains(secondText, "FirstValue") {
		t.Fatalf("cross-contaminated intermediates:\nfirst=%s\nsecond=%s", firstText, secondText)
	}
}

func TestRunConfigControlsSharedTLAStatisticsMarker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"config"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("config = %d, stderr = %q", code, &stderr)
	}
	configDir := filepath.Join(home, ".tlaplus")
	if info, err := os.Stat(configDir); err != nil || !info.IsDir() {
		t.Fatalf("configuration directory = %v, %v", info, err)
	}
	marker := filepath.Join(configDir, "esc.txt")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("config without an option changed the marker: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"config", "--enable-stats=true"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("enable config = %d, stderr = %q", code, &stderr)
	}
	contents, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}\n$`).Match(contents) {
		t.Fatalf("opt-in marker = %q", contents)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"config", "--enable-stats=false"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("disable config = %d, stderr = %q", code, &stderr)
	}
	contents, err = os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(contents), "NO_STATISTICS\n"; got != want {
		t.Fatalf("opt-out marker = %q, want %q", got, want)
	}
}

func TestRunConfigRejectsInvalidStatisticsChoice(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"config", "--enable-stats=maybe"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "must be true or false") {
		t.Fatalf("config = %d, stderr = %q", code, &stderr)
	}
}

func TestRunConfigAcceptsScalaOptionalBooleanForms(t *testing.T) {
	for _, test := range []struct {
		argument string
		enabled  bool
	}{
		{"--enable-stats", true},
		{"--enable-stats=yes", true},
		{"--enable-stats=1", true},
		{"--enable-stats=no", false},
		{"--enable-stats=0", false},
	} {
		t.Run(test.argument, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			var stdout, stderr bytes.Buffer
			if code := run(context.Background(), []string{"config", test.argument}, &stdout, &stderr); code != exitOK {
				t.Fatalf("config = %d, stderr = %q", code, &stderr)
			}
			contents, err := os.ReadFile(filepath.Join(home, ".tlaplus", "esc.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if test.enabled {
				if !regexp.MustCompile(`^[0-9a-f]{32}\n$`).Match(contents) {
					t.Fatalf("enabled marker = %q", contents)
				}
			} else if string(contents) != "NO_STATISTICS\n" {
				t.Fatalf("disabled marker = %q", contents)
			}
		})
	}
}

func TestRunConfigRejectsArgumentsAndConfigurationDirectoryFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"config", "unexpected"}, &stdout, &stderr); code != exitUsage ||
		!strings.Contains(stderr.String(), "does not accept positional arguments") {
		t.Fatalf("positional config = %d, stderr = %q", code, &stderr)
	}
	if err := os.WriteFile(filepath.Join(home, ".tlaplus"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"config", "--enable-stats"}, &stdout, &stderr); code != exitTool ||
		!strings.Contains(stderr.String(), "cannot create TLA+ configuration directory") {
		t.Fatalf("directory failure = %d, stderr = %q", code, &stderr)
	}
}

func TestRunServerStartsExplorerAndStopsWithContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PORT", "not-used-because-cli-wins")
	originalListen := listenTCP
	listenTCP = func(network, address string) (net.Listener, error) {
		if network != "tcp" || address != ":0" {
			t.Fatalf("listen = %q, %q", network, address)
		}
		return &testListener{address: &net.TCPAddr{Port: 43210}}, nil
	}
	t.Cleanup(func() { listenTCP = originalListen })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"server", "--port=0", "--server-type=explorer"}, &stdout, &stderr)
	if code != exitOK || !strings.Contains(stdout.String(), "JSON-RPC explorer server running on http://localhost:") {
		t.Fatalf("server = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunServerUsesPORTEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PORT", "0")
	originalListen := listenTCP
	listenTCP = func(network, address string) (net.Listener, error) {
		if network != "tcp" || address != ":0" {
			t.Fatalf("listen = %q, %q", network, address)
		}
		return &testListener{address: &net.TCPAddr{Port: 43211}}, nil
	}
	t.Cleanup(func() { listenTCP = originalListen })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	if code := run(ctx, []string{"server"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("server with PORT = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunServerRejectsInvalidPORTEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PORT", "not-a-port")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"server"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "invalid PORT environment") {
		t.Fatalf("server with invalid PORT = %d, stderr = %q", code, &stderr)
	}
}

func TestExplorationHTTPServerPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := newExplorationHTTPServer(ctx)
	requestContext := server.BaseContext(nil)
	cancel()
	select {
	case <-requestContext.Done():
	case <-time.After(time.Second):
		t.Fatal("server request context did not inherit cancellation")
	}
}

type testListener struct {
	address net.Addr
}

func (listener *testListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (listener *testListener) Close() error              { return nil }
func (listener *testListener) Addr() net.Addr            { return listener.address }

func TestRunServerStartsCheckerAndRejectsUnknownService(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	originalListen := listenTCP
	listenTCP = func(network, address string) (net.Listener, error) {
		return &testListener{address: &net.TCPAddr{Port: 43212}}, nil
	}
	t.Cleanup(func() { listenTCP = originalListen })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"server", "--port=0", "--server-type=checker"}, &stdout, &stderr)
	if code != exitOK || !strings.Contains(stdout.String(), "gRPC checker server running") {
		t.Fatalf("server = %d, stderr = %q", code, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"server", "--server-type=unknown"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "want explorer or checker") {
		t.Fatalf("unknown server = %d, stderr = %q", code, &stderr)
	}
}

func TestRunCheckerServerServesGRPCAndStops(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	buffered := bufconn.Listen(1 << 20)
	listener := &addressedListener{Listener: buffered, address: &net.TCPAddr{Port: 43213}}
	originalListen := listenTCP
	listenTCP = func(network, address string) (net.Listener, error) { return listener, nil }
	t.Cleanup(func() { listenTCP = originalListen })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	var stdout, stderr bytes.Buffer
	go func() {
		done <- run(ctx, []string{"server", "--port=0", "--server-type=checker"}, &stdout, &stderr)
	}()
	connection, err := grpc.NewClient("passthrough:///checker",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return buffered.Dial() }),
	)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer connection.Close()
	rpcContext, stopRPC := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopRPC()
	if _, err = cmd.NewCmdExecutorClient(connection).Ping(rpcContext, &cmd.PingRequest{}); err != nil {
		cancel()
		t.Fatalf("checker Ping = %v", err)
	}
	cancel()
	select {
	case code := <-done:
		if code != exitOK {
			t.Fatalf("checker server = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("checker server did not stop after cancellation")
	}
}

type addressedListener struct {
	net.Listener
	address net.Addr
}

func (listener *addressedListener) Addr() net.Addr { return listener.address }

func TestRunTraceeEvaluatesExpressionsToITF(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "TraceEval.tla")
	tracePath := filepath.Join(dir, "input.itf.json")
	if err := os.WriteFile(sourcePath, []byte(`---- MODULE TraceEval ----
EXTENDS Integers
\* @type: Int;
CONSTANT N
\* @type: Int;
VARIABLE x
Square == x * x
Even == x % 2 = 0
PlusN == x + N
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracePath, []byte(`{
  "#meta":{"format":"ITF","varTypes":{"x":"Int"}},
  "params":["N"],
  "vars":["x"],
  "states":[{"N":10,"x":2},{"x":3}]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"tracee", "-trace", tracePath, "-expressions", "Square,Even,PlusN", sourcePath,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("tracee = %d, stderr = %q", code, &stderr)
	}
	states, err := pal.UnmarshalITFTrace(stdout.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatalf("evaluated states = %#v", states)
	}
	firstSquare, _ := pal.FormatTLAExpression(states[0]["Square"])
	secondSquare, _ := pal.FormatTLAExpression(states[1]["Square"])
	firstEven, _ := pal.FormatTLAExpression(states[0]["Even"])
	secondEven, _ := pal.FormatTLAExpression(states[1]["Even"])
	firstPlusN, _ := pal.FormatTLAExpression(states[0]["PlusN"])
	secondPlusN, _ := pal.FormatTLAExpression(states[1]["PlusN"])
	if firstSquare != "4" || secondSquare != "9" || firstEven != "TRUE" || secondEven != "FALSE" {
		t.Fatalf("evaluated states = %#v", states)
	}
	if firstPlusN != "12" || secondPlusN != "13" {
		t.Fatalf("constant evaluation = %q, %q", firstPlusN, secondPlusN)
	}

	stdout.Reset()
	stderr.Reset()
	runDir := filepath.Join(dir, "tracee-run")
	code = run(context.Background(), []string{
		"tracee", "--run-dir=" + runDir, "-trace", tracePath, "-expressions", "Square,Even,PlusN", sourcePath,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run-dir tracee = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "example.itf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if evaluated, decodeErr := pal.UnmarshalITFTrace(data); decodeErr != nil || len(evaluated) != 2 {
		t.Fatalf("run-dir evaluated trace = %#v, %v", evaluated, decodeErr)
	}
	if _, err = os.Stat(filepath.Join(runDir, "run.txt")); err != nil {
		t.Fatalf("tracee run metadata: %v", err)
	}
}

func TestRunTraceeAcceptsScalaPositionalInterface(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "TraceEval.tla")
	tracePath := filepath.Join(dir, "input.itf.json")
	if err := os.WriteFile(sourcePath, []byte(`---- MODULE TraceEval ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Double == x + x
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracePath, []byte(`{
  "#meta":{"format":"ITF","varTypes":{"x":"Int"}},
  "vars":["x"],"states":[{"x":4}]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"tracee", sourcePath, tracePath, "Double"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("positional tracee = %d, stderr = %q", code, &stderr)
	}
	states, err := pal.UnmarshalITFTrace(stdout.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	value, err := pal.FormatTLAExpression(states[0]["Double"])
	if err != nil || value != "8" {
		t.Fatalf("Double = %q, %v", value, err)
	}
}

func TestRunTranspileGeneratesStandaloneVMT(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "VMT.tla")
	if err := os.WriteFile(sourcePath, []byte(`---- MODULE VMT ----
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x
Inv == x = x
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"transpile", "-inv=Inv", "-output=-", sourcePath,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("transpile = %d, stderr = %q", code, &stderr)
	}
	for _, fragment := range []string{
		";Variables\n(declare-fun x () Int)",
		"(define-fun .x () Int (! x :next x^))",
		":init true",
		":action Next$",
		":invar-property 0",
	} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Errorf("VMT stdout does not contain %q:\n%s", fragment, &stdout)
		}
	}

	stdout.Reset()
	stderr.Reset()
	outputPath := filepath.Join(dir, "model.vmt")
	code = run(context.Background(), []string{
		"transpile", "-inv=Inv", "-output", outputPath, sourcePath,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("file transpile = %d, stderr = %q", code, &stderr)
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil || !strings.Contains(string(contents), ";Initial states") {
		t.Fatalf("written VMT = %q, %v", contents, err)
	}
	if !strings.Contains(stdout.String(), outputPath) {
		t.Fatalf("transpile success message = %q", &stdout)
	}

	outDir := filepath.Join(dir, "transpile-out")
	runDir := filepath.Join(dir, "transpile-run")
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{
		"transpile", "-inv=Inv", "--out-dir=" + outDir, "--run-dir=" + runDir, sourcePath,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("directory transpile = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	if _, err = os.Stat(filepath.Join(runDir, "output.vmt")); err != nil {
		t.Fatalf("custom-run VMT: %v", err)
	}
	generated, err := filepath.Glob(filepath.Join(outDir, filepath.Base(sourcePath), "*", "output.vmt"))
	if err != nil || len(generated) != 1 {
		t.Fatalf("generated-run VMT = %v, %v", generated, err)
	}
}

func TestRunTranspileValidatesArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"transpile", "-init=Init", "M.tla"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "must be provided together") {
		t.Fatalf("transpile = %d, stderr = %q", code, &stderr)
	}
}

func TestRunTranspileLowersReTLAFunctionsEndToEnd(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "Functions.tla")
	if err := os.WriteFile(sourcePath, []byte(`---- MODULE Functions ----
\* @type: Set(A);
CONSTANT S
\* @type: A -> A;
VARIABLE f
Init == f = [x \in S |-> "a_OF_A"]
Next == f' = [f EXCEPT !["a_OF_A"] = "b_OF_A"]
Inv == f["a_OF_A"] = "a_OF_A" \/ f["a_OF_A"] = "b_OF_A"
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"transpile", "-inv=Inv", "-output=-", sourcePath,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("function transpile = %d, stderr = %q", code, &stderr)
	}
	for _, fragment := range []string{
		"(declare-sort A 0)",
		"(declare-fun f (A) A)",
		"(define-fun t_",
		"(ite (= ",
		"(f a_A)",
		"b_A",
	} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Errorf("function VMT does not contain %q:\n%s", fragment, &stdout)
		}
	}
}

func TestRunParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Tiny.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE Tiny ----
Value == 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"parse", path}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run() = %d, stderr = %s", code, &stderr)
	}
	if got, want := stdout.String(), "parsed module Tiny\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunParseJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Tiny.tla")
	if err := os.WriteFile(path, []byte("---- MODULE Tiny ----\nValue == 1\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"parse", "-output=json", path}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run() = %d, stderr = %s", code, &stderr)
	}
	if !strings.Contains(stdout.String(), `"name": "ApalacheIR"`) {
		t.Fatalf("stdout does not contain ApalacheIR: %s", &stdout)
	}
}

func TestRunParseAndTypecheckWriteRoundTrippableFiles(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "RoundTrip.tla")
	if err := os.WriteFile(sourcePath, []byte(`---- MODULE RoundTrip ----
EXTENDS Integers
\* @type: Int;
CONSTANT N
Value == N + 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"parse", "typecheck"} {
		t.Run(command, func(t *testing.T) {
			outputDir := filepath.Join(dir, command)
			if err := os.Mkdir(outputDir, 0o700); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(outputDir, "RoundTrip.tla")
			runDir := filepath.Join(outputDir, "run")
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{
				command, "--run-dir=" + runDir, "-output", outputPath, sourcePath,
			}, &stdout, &stderr)
			if code != exitOK {
				t.Fatalf("%s output = %d, stderr = %q", command, code, &stderr)
			}
			contents, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(contents, []byte("EXTENDS Integers")) || !bytes.Contains(contents, []byte("@type: Int;")) {
				t.Fatalf("%s output lost module metadata:\n%s", command, contents)
			}
			result := pal.ParseAndTypeCheckFile(context.Background(), outputPath, pal.FrontendOptions{}, pal.TypeCheckOptions{})
			if !result.OK() {
				t.Fatalf("%s output did not round trip: %v\n%s", command, result.Diagnostics, contents)
			}
			if data, err := os.ReadFile(filepath.Join(runDir, "run.txt")); err != nil ||
				!strings.Contains(string(data), command+" --run-dir=") {
				t.Fatalf("%s run metadata = %q, %v", command, data, err)
			}
		})
	}

	jsonPath := filepath.Join(dir, "typed.json")
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"typecheck", "-output", jsonPath, sourcePath}, &stdout, &stderr); code != exitOK {
		t.Fatalf("JSON file output = %d, stderr = %q", code, &stderr)
	}
	var document map[string]any
	data, err := os.ReadFile(jsonPath)
	if err != nil || json.Unmarshal(data, &document) != nil || document["name"] != "ApalacheIR" {
		t.Fatalf("JSON file output = %v, %v, %s", document, err, data)
	}
}

func TestRunParseFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bad.tla")
	if err := os.WriteFile(path, []byte("---- MODULE Bad ----\nValue == missing\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"parse", path}, &stdout, &stderr)
	if code != exitFrontend {
		t.Fatalf("run() = %d, want %d; stderr = %s", code, exitFrontend, &stderr)
	}
	if stderr.Len() == 0 {
		t.Fatal("run() emitted no diagnostic")
	}
}

func TestRunAcceptsScalaFeaturesOption(t *testing.T) {
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "Empty.tla")
	if err := os.WriteFile(emptyPath, []byte("---- MODULE Empty ----\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, feature := range []string{"rows", "no-rows"} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), []string{"parse", "--features=" + feature, emptyPath}, &stdout, &stderr); code != exitOK {
			t.Fatalf("parse feature %s = %d, stderr = %q", feature, code, &stderr)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"parse", "--features=unsupported", emptyPath}, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), "unsupported feature") {
		t.Fatalf("unsupported parse feature = %d, stderr = %q", code, &stderr)
	}

	rowsPath := filepath.Join(dir, "Rows.tla")
	if err := os.WriteFile(rowsPath, []byte(`---- MODULE Rows ----
\* @type: { a: b, c } => b;
GetA(r) == r.a
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"typecheck", "--features=rows", rowsPath}, &stdout, &stderr); code != exitOK {
		t.Fatalf("row feature was not propagated = %d, stderr = %q", code, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"typecheck", "--features=no-rows", rowsPath}, &stdout, &stderr); code != exitFrontend || !strings.Contains(stderr.String(), "Found row types") {
		t.Fatalf("no-rows feature was not propagated = %d, stderr = %q", code, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"typecheck", "-rows", "--features=no-rows", rowsPath}, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), "conflicts") {
		t.Fatalf("row feature conflict = %d, stderr = %q", code, &stderr)
	}
}

func TestRunTypeCheckAndTypedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Typed.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE Typed ----
EXTENDS Integers
\* @type: Int;
CONSTANT N
Value == N + 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"typecheck", path}, &stdout, &stderr); code != exitOK {
		t.Fatalf("run() = %d, stderr = %s", code, &stderr)
	}
	if got, want := stdout.String(), "type-checked module Typed\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"typecheck", "-output=json", path}, &stdout, &stderr); code != exitOK {
		t.Fatalf("JSON run() = %d, stderr = %s", code, &stderr)
	}
	if !strings.Contains(stdout.String(), `"type": "Int"`) || strings.Contains(stdout.String(), `"type": "Untyped"`) {
		t.Fatalf("stdout is not fully typed IR: %s", &stdout)
	}
}

func TestRunTypeCheckReportsMissingAnnotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Untyped.tla")
	if err := os.WriteFile(path, []byte("---- MODULE Untyped ----\nCONSTANT N\nValue == N\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"typecheck", path}, &stdout, &stderr)
	if code != exitFrontend || !strings.Contains(stderr.String(), "Expected a type annotation for CONSTANT N") {
		t.Fatalf("run() = %d, stderr = %q", code, &stderr)
	}
}

func TestRunCheckFindsCounterexampleAndHonorsBound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Counter.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE Counter ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
Inv == x < 2
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	direct := []string{"check", "--algo=incremental", "--smt-encoding=oopsla19", "--smt-solver=z3", "-inv=Inv", "-length=1", path}
	if code := run(context.Background(), direct, &stdout, &stderr); code != exitOK {
		t.Fatalf("safe check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	if !strings.Contains(stdout.String(), "No counterexample found through depth 1") {
		t.Fatalf("safe output = %q", &stdout)
	}
	stdout.Reset()
	stderr.Reset()
	delayedPruning := append([]string{"check", "--discard-disabled=false"}, direct[1:]...)
	if code := run(context.Background(), delayedPruning, &stdout, &stderr); code != exitOK {
		t.Fatalf("non-eager disabled transition check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	safeTracePath := filepath.Join(dir, "example.itf.json")
	stdout.Reset()
	stderr.Reset()
	safeTraceArgs := append([]string{"check", "--output-traces", "--itf=" + safeTracePath}, direct[1:]...)
	if code := run(context.Background(), safeTraceArgs, &stdout, &stderr); code != exitOK {
		t.Fatalf("safe trace check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	if data, err := os.ReadFile(safeTracePath); err != nil {
		t.Fatal(err)
	} else if states, decodeErr := pal.UnmarshalITFTrace(data); decodeErr != nil || len(states) != 2 {
		t.Fatalf("safe example trace = %d states, %v", len(states), decodeErr)
	}
	outDir := filepath.Join(dir, "out")
	runDir := filepath.Join(dir, "custom-run")
	stdout.Reset()
	stderr.Reset()
	directoryTraceArgs := append([]string{"check", "--output-traces", "--out-dir=" + outDir, "--run-dir=" + runDir}, direct[1:]...)
	if code := run(context.Background(), directoryTraceArgs, &stdout, &stderr); code != exitOK {
		t.Fatalf("directory trace check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	if _, err := os.Stat(filepath.Join(runDir, "example.itf.json")); err != nil {
		t.Fatalf("custom run trace: %v", err)
	}
	legacyData, err := os.ReadFile(filepath.Join(runDir, "example.json"))
	if err != nil {
		t.Fatalf("custom legacy trace: %v", err)
	}
	if legacyStates, decodeErr := pal.UnmarshalApalacheJSONTrace(legacyData); decodeErr != nil || len(legacyStates) != 2 {
		t.Fatalf("custom legacy trace states = %#v, %v", legacyStates, decodeErr)
	}
	if tlaData, err := os.ReadFile(filepath.Join(runDir, "example.tla")); err != nil ||
		!bytes.Contains(tlaData, []byte("State1")) {
		t.Fatalf("custom TLA trace = %q, %v", tlaData, err)
	}
	if tlcData, err := os.ReadFile(filepath.Join(runDir, "MCexample.out")); err != nil ||
		!bytes.Contains(tlcData, []byte("2: <Next>")) {
		t.Fatalf("custom TLC trace = %q, %v", tlcData, err)
	}
	if data, err := os.ReadFile(filepath.Join(runDir, "run.txt")); err != nil ||
		!strings.Contains(string(data), "check --output-traces") || !strings.Contains(string(data), path) {
		t.Fatalf("custom run metadata = %q, %v", data, err)
	}
	generated, err := filepath.Glob(filepath.Join(outDir, filepath.Base(path), "*", "example.itf.json"))
	if err != nil || len(generated) != 1 {
		t.Fatalf("generated run traces = %v, %v", generated, err)
	}
	jsonPath := filepath.Join(dir, "Counter.json")
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"typecheck", "-output", jsonPath, path}, &stdout, &stderr); code != exitOK {
		t.Fatalf("write typed JSON = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), append(append([]string(nil), direct[:len(direct)-1]...), jsonPath), &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "No counterexample found through depth 1") {
		t.Fatalf("JSON check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}

	stdout.Reset()
	stderr.Reset()
	itfPath := filepath.Join(dir, "counterexample.itf.json")
	unsafeArgs := []string{"check", "-init=Init", "-next=Next", "-inv=Inv", "-length=2", "-itf=" + itfPath, path}
	if code := run(context.Background(), unsafeArgs, &stdout, &stderr); code != exitCounterexample {
		t.Fatalf("unsafe check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	if !strings.Contains(stdout.String(), "Invariant Inv violated at depth 2") ||
		!strings.Contains(stdout.String(), "Init$0000, Next$0000, Next$0000") ||
		!strings.Contains(stdout.String(), "State 0: x = 0") ||
		!strings.Contains(stdout.String(), "State 2: x = 2") {
		t.Fatalf("counterexample output = %q", &stdout)
	}
	data, err := os.ReadFile(itfPath)
	if err != nil {
		t.Fatal(err)
	}
	var trace map[string]any
	if err = json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	if states, ok := trace["states"].([]any); !ok || len(states) != 3 {
		t.Fatalf("ITF counterexample = %#v", trace)
	}
}

func TestRunCheckSupportsNativeArrayEncodings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "NativeArrays.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE NativeArrays ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Native == LET f == [i \in {1} |-> {i, i + 1}] IN 2 \in f[1]
Init == x = 0 /\ Native
Next == x' = x
Inv == Native
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, encoding := range []string{"arrays", "funArrays"} {
		t.Run(encoding, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{
				"check", "--smt-encoding=" + encoding, "--no-deadlock", "--inv=Inv", "--length=1", path,
			}, &stdout, &stderr)
			if code != exitOK || !strings.Contains(stdout.String(), "No counterexample found through depth 1") {
				t.Fatalf("native %s check = %d, stdout = %q, stderr = %q", encoding, code, &stdout, &stderr)
			}
			stdout.Reset()
			stderr.Reset()
			code = run(context.Background(), []string{
				"simulate", "--smt-encoding=" + encoding, "--max-run=1", "--length=1", path,
			}, &stdout, &stderr)
			if code != exitOK || !strings.Contains(stdout.String(), "Simulation completed through depth 1") {
				t.Fatalf("native %s simulation = %d, stdout = %q, stderr = %q", encoding, code, &stdout, &stderr)
			}
		})
	}
}

func TestRunCheckLoadsTLCConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Configured.tla")
	configPath := filepath.Join(dir, "Configured.cfg")
	if err := os.WriteFile(path, []byte(`---- MODULE Configured ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x
Inv == x = 0
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("INIT Init\nNEXT Next\nINVARIANT Inv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "-config", configPath, "-length=1", path}, &stdout, &stderr)
	if code != exitOK || !strings.Contains(stdout.String(), "No counterexample") {
		t.Fatalf("configured check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunCheckFindsTemporalLassoCounterexample(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Liveness.tla")
	configPath := filepath.Join(dir, "Liveness.cfg")
	if err := os.WriteFile(path, []byte(`---- MODULE Liveness ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = IF x = 0 THEN 1 ELSE 0
Live == <> (x = 2)
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("INIT Init\nNEXT Next\nPROPERTY Live\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "-config", configPath, "-length=3", path}, &stdout, &stderr)
	if code != exitCounterexample || !strings.Contains(stdout.String(), "Invariant Live violated") ||
		strings.Contains(stdout.String(), "__") {
		t.Fatalf("temporal check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunCheckAcceptsDirectCommaSeparatedProperties(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Direct.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE Direct ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x
Inv == x = 0
Live == <> (x = 1)
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"check", "-init=Init", "-next=Next", "-deadlock=false",
		"-inv=Inv", "-temporal=Live", "-length=1", path,
	}, &stdout, &stderr)
	if code != exitCounterexample || !strings.Contains(stdout.String(), "Invariant Live violated") {
		t.Fatalf("direct temporal check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}

	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{
		"check", "-init=Init", "-next=Next", "-deadlock=false",
		"-inv=Inv,Inv", "-length=0", path,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("comma invariant check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunCheckReportsDeadlockWithoutInvariant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Deadlock.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE Deadlock ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == /\ x' = x + 1
        /\ x < 0
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"check", "-init=Init", "-next=Next", "-length=1", path,
	}, &stdout, &stderr)
	if code != exitCounterexample || !strings.Contains(stdout.String(), "Deadlock found at depth 0") ||
		!strings.Contains(stdout.String(), "State 0: x = 0") {
		t.Fatalf("deadlock check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunCheckAcceptsScalaNoDeadlockAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "NoDeadlock.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE NoDeadlock ----
Init == TRUE
Next == FALSE
Inv == TRUE
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "--no-deadlock", "--inv=Inv", "--length=0", path}, &stdout, &stderr)
	if code != exitOK || !strings.Contains(stdout.String(), "No counterexample") {
		t.Fatalf("no-deadlock alias = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"check", "--no-deadlock", "--deadlock=true", path}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "conflicts") {
		t.Fatalf("deadlock conflict = %d, stderr = %q", code, &stderr)
	}
}

func TestRunSimulateWritesDecodedAndITFTrace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Sim.tla")
	itfPath := filepath.Join(dir, "sim.itf.json")
	if err := os.WriteFile(path, []byte(`---- MODULE Sim ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"simulate", "--algo=incremental", "--smt-encoding=oopsla19", "--smt-solver=z3",
		"-length=2", "-itf=" + itfPath, path,
	}, &stdout, &stderr)
	if code != exitOK || !strings.Contains(stdout.String(), "Simulation completed through depth 2") ||
		!strings.Contains(stdout.String(), "State 2: x = 2") {
		t.Fatalf("simulate = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	data, err := os.ReadFile(itfPath)
	if err != nil {
		t.Fatal(err)
	}
	var trace map[string]any
	if err = json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	if states, ok := trace["states"].([]any); !ok || len(states) != 3 {
		t.Fatalf("ITF simulation = %#v", trace)
	}
	stdout.Reset()
	stderr.Reset()
	implicitRunDir := filepath.Join(dir, "simulation-run")
	code = run(context.Background(), []string{
		"simulate", "--max-run=1", "--length=1", "--output-traces", "--run-dir=" + implicitRunDir, path,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run-dir simulation = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	if _, err = os.Stat(filepath.Join(implicitRunDir, "example.itf.json")); err != nil {
		t.Fatalf("run-dir simulation trace: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	multiPath := filepath.Join(dir, "multi.itf.json")
	code = run(context.Background(), []string{"simulate", "--max-run=2", "--length=1", "--itf=" + multiPath, path},
		&stdout, &stderr)
	if code != exitOK {
		t.Fatalf("multi-run ITF = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	for run := 1; run <= 2; run++ {
		outputPath := filepath.Join(dir, fmt.Sprintf("multi.itf-run-%d.json", run))
		if _, err = os.Stat(outputPath); err != nil {
			t.Fatalf("numbered ITF run %d: %v", run, err)
		}
	}
}

func TestRunCheckerCommandsValidateSMTTimeout(t *testing.T) {
	for _, command := range []string{"check", "simulate"} {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{command, "--timeout-smt=-1", "ignored.tla"}, &stdout, &stderr)
		if code != exitUsage || !strings.Contains(stderr.String(), "SMT timeout cannot be negative") {
			t.Fatalf("%s negative timeout = %d, stderr = %q", command, code, &stderr)
		}
	}
}

func TestRunTestChecksAssertionOnlyAfterAction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "OneStep.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE OneStep ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Before == x = 0
Increment == x' = x + 1
Positive == x > 0
StillZero == x = 0
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"test", path, "Before", "Increment", "Positive"}
	if code := run(context.Background(), args, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "No example found") {
		t.Fatalf("passing test = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	args[4] = "StillZero"
	if code := run(context.Background(), args, &stdout, &stderr); code != exitCounterexample ||
		!strings.Contains(stdout.String(), "Assertion StillZero violated after Increment") ||
		!strings.Contains(stdout.String(), "State 1: x = 1") {
		t.Fatalf("failing test = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	runDir := filepath.Join(dir, "test-run")
	args = []string{"test", "--run-dir=" + runDir, path, "Before", "Increment", "StillZero"}
	if code := run(context.Background(), args, &stdout, &stderr); code != exitCounterexample {
		t.Fatalf("failing output test = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	for _, name := range []string{"run.txt", "violation.tla", "MCviolation.out", "violation.json", "violation.itf.json"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Fatalf("test output %s: %v", name, err)
		}
	}
}

func TestRunCheckValidatesConfigurationFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "-init=Init", "M.tla"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "-init and -next") {
		t.Fatalf("partial behavior flags = %d, stderr = %q", code, &stderr)
	}
}

func TestRunCheckerCommandsRejectUnsupportedBackends(t *testing.T) {
	for _, test := range []struct {
		command, option, message string
	}{
		{"check", "--algo=offline", "supports incremental"},
		{"check", "--smt-encoding=unknown", "want oopsla19, arrays, or funArrays"},
		{"check", "--smt-solver=cvc5", "g3 v0.0.5"},
		{"simulate", "--algo=remote", "supports incremental"},
	} {
		t.Run(test.command+" "+test.option, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{test.command, test.option, "M.tla"}, &stdout, &stderr)
			if code != exitUsage || !strings.Contains(stderr.String(), test.message) {
				t.Fatalf("run() = %d, stderr = %q", code, &stderr)
			}
		})
	}
}

func TestCheckerBackendEnvironmentPrecedence(t *testing.T) {
	t.Setenv("SMT_ENCODING", "arrays")
	t.Setenv("SMT_SOLVER", "z3")
	options := &checkerBackendOptions{algorithm: "incremental", encoding: "oopsla19", solver: "z3"}
	visited := make(map[string]bool)
	applyCheckerBackendEnvironment(options, visited)
	if options.encoding != "arrays" || options.solver != "z3" ||
		!visited["smt-encoding"] || !visited["smt-solver"] {
		t.Fatalf("environment backend = %#v, visited=%v", options, visited)
	}

	options.encoding = "funArrays"
	options.solver = "explicit"
	applyCheckerBackendEnvironment(options, visited)
	if options.encoding != "funArrays" || options.solver != "explicit" {
		t.Fatalf("explicit backend was overwritten by environment: %#v", options)
	}
}

func TestRunCheckerCommandsValidateBackendEnvironment(t *testing.T) {
	t.Setenv("SMT_ENCODING", "unknown")
	for _, command := range []string{"check", "simulate"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{command, "M.tla"}, &stdout, &stderr)
			if code != exitUsage || !strings.Contains(stderr.String(), "want oopsla19, arrays, or funArrays") {
				t.Fatalf("%s environment validation = %d, stderr=%q", command, code, &stderr)
			}
		})
	}
}

func TestParseTuningOptionsMatchesScalaCLIFormat(t *testing.T) {
	got, err := parseTuningOptions("search.invariant.mode=after:z3.smt.mbqi=false:value=with spaces")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"search.invariant.mode": "after",
		"z3.smt.mbqi":           "false",
		"value":                 "with spaces",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tuning options = %#v, want %#v", got, want)
	}
	for _, invalid := range []string{"missing", "=value", "key=", "key=a=b"} {
		if _, err = parseTuningOptions(invalid); err == nil {
			t.Fatalf("accepted invalid tuning options %q", invalid)
		}
	}
}

func TestLoadTuningProperties(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checker.properties")
	contents := "# checker settings\nsearch.invariant.mode = before\n! ignored\nz3.smt.mbqi:false\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadTuningProperties(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"search.invariant.mode": "before", "z3.smt.mbqi": "false"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tuning properties = %#v, want %#v", got, want)
	}
	bad := filepath.Join(t.TempDir(), "bad.properties")
	if err = os.WriteFile(bad, []byte("missing separator\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = loadTuningProperties(bad); err == nil {
		t.Fatal("accepted malformed tuning properties")
	}
}

func TestRunUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), nil, &stdout, &stderr); code != exitUsage {
		t.Fatalf("run() = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q", &stderr)
	}
}
