package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	pal "github.com/glycerine/apalache-go"
)

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

func TestRunServerRejectsUnsupportedCheckerService(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"server", "--server-type=checker"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "currently supports explorer") {
		t.Fatalf("server = %d, stderr = %q", code, &stderr)
	}
}

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
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{command, "-output", outputPath, sourcePath}, &stdout, &stderr)
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
	if code != exitFrontend || !strings.Contains(stderr.String(), "expected a type annotation for CONSTANT N") {
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
		{"check", "--smt-encoding=arrays", "supports oopsla19"},
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

func TestRunUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), nil, &stdout, &stderr); code != exitUsage {
		t.Fatalf("run() = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q", &stderr)
	}
}
