package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pal "github.com/glycerine/apalache-go"
)

func TestLoadCommandConfigIsStrict(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		name, document, want string
	}{
		{"unknown key", `{"checker":{"length":1,"mystery":true}}`, "unknown field"},
		{"unknown source key", `{"source":{"kind":"string","content":"---- MODULE M ----\n====","mystery":true}}`, "unknown field"},
		{"duplicate key", `{"checker":{"length":1,"length":2}}`, "duplicate key"},
		{"trailing value", `{"checker":{"length":1}} {}`, "trailing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(directory, strings.ReplaceAll(test.name, " ", "-")+".json")
			if err := os.WriteFile(path, []byte(test.document), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := loadCommandConfig(path); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("loadCommandConfig error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestMaterializeSpecificationPreservesQuintFormat(t *testing.T) {
	kind, format, content := "string", "qnt", `{"modules":[],"types":{}}`
	source := &jsonInputSource{Kind: &kind, Format: &format, Content: &content}
	path, cleanup, err := source.materializeSpecification()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if !strings.HasSuffix(path, ".qnt.json") {
		t.Fatalf("materialized Quint path = %q", path)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != content {
		t.Fatalf("materialized Quint content = %q, %v", data, readErr)
	}
}

func TestJSONModelConfigAcceptsBothDisabledTransitionStrategies(t *testing.T) {
	for _, command := range []struct{ name string }{
		{"check"}, {"simulate"}, {"test"},
	} {
		t.Run(command.name, func(t *testing.T) {
			for _, discardDisabled := range []bool{false, true} {
				config := jsonCommandConfig{Checker: jsonCheckerConfig{DiscardDisabled: &discardDisabled}}
				var err error
				switch command.name {
				case "check":
					err = config.validateForCheck()
				case "simulate":
					err = config.validateForSimulate()
				default:
					err = config.validateForModelCommand("test", false)
				}
				if err != nil {
					t.Fatalf("discard-disabled=%v validation = %v", discardDisabled, err)
				}
			}
		})
	}
}

func TestJSONModelConfigValidatesSupportedSearchTuning(t *testing.T) {
	supported := map[string]string{
		"search.transitionFilter": "(0->0|[1-9]->.*)",
		"search.invariantFilter":  "[1-9]->state0",
		"search.invariant.mode":   "after",
		"smt.randomSeed":          "7",
		"z3.smt.random_seed":      "9",
		"z3.smt.mbqi":             "true",
		"z3.sat.random_freq":      "0.25",
		"search.outputTraces":     "true",
	}
	if err := (jsonCommandConfig{Checker: jsonCheckerConfig{Tuning: &supported}}).validateForCheck(); err != nil {
		t.Fatalf("supported tuning = %v", err)
	}
	unsupported := map[string]string{"z3.not_a_parameter": "true"}
	err := (jsonCommandConfig{Checker: jsonCheckerConfig{Tuning: &unsupported}}).validateForCheck()
	if err == nil || !strings.Contains(err.Error(), "not_a_parameter") {
		t.Fatalf("unsupported tuning = %v", err)
	}
}

func TestJSONServerConfigAcceptsOnlyServerSettings(t *testing.T) {
	port := 0
	explorer := "explorer"
	if err := (jsonCommandConfig{Server: jsonServerConfig{Port: &port, ServerType: &explorer}}).validateForServer(); err != nil {
		t.Fatalf("server-only configuration = %v", err)
	}
	length := 1
	err := (jsonCommandConfig{Checker: jsonCheckerConfig{Length: &length}}).validateForServer()
	if err == nil || !strings.Contains(err.Error(), "checker") {
		t.Fatalf("checker server configuration = %v", err)
	}
}

func TestJSONCommonOutputControlsAreAcceptedByFileCommands(t *testing.T) {
	enabled := true
	config := jsonCommandConfig{Debug: &enabled, WriteIntermediate: &enabled}
	if err := config.validateForFrontend("parse"); err != nil {
		t.Fatalf("parse write-intermediate configuration = %v", err)
	}
	if err := config.validateForFrontend("typecheck"); err != nil {
		t.Fatalf("typecheck common output configuration = %v", err)
	}
	if err := config.validateForCheck(); err != nil {
		t.Fatalf("check common output configuration = %v", err)
	}
}

func TestRunCheckLoadsInMemoryJSONSourceWithAuxiliaryModule(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	mainSource := `---- MODULE ConfiguredMemory ----
EXTENDS Aux, Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = Inc(x)
Inv == x < 2
====
`
	auxiliary := `---- MODULE Aux ----
EXTENDS Integers
\* @type: Int => Int;
Inc(x) == x + 1
====
`
	configuration, err := json.Marshal(map[string]any{
		"source": map[string]any{
			"kind": "string", "content": mainSource, "aux": []string{auxiliary}, "format": "tla",
		},
		"checker": map[string]any{"length": 2, "inv": []string{"Inv"}, "no-deadlock": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("memory.json", configuration, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"check", "--config-file=memory.json"}, &stdout, &stderr); code != exitCounterexample ||
		!strings.Contains(stdout.String(), "Invariant Inv violated at depth 2") {
		t.Fatalf("in-memory configured check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunCheckCollectsConfiguredDistinctCounterexamples(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	source := `---- MODULE MultiConfigured ----
EXTENDS Integers, FiniteSets
\* @type: Int;
VARIABLE x
Init == x \in {1, 2, 3}
Next == x' = x
Inv == x = 0
View == x
====
`
	document, err := json.Marshal(map[string]any{
		"source": map[string]any{"kind": "string", "content": source, "format": "tla"},
		"checker": map[string]any{
			"length": 0, "inv": []string{"Inv"}, "view": "View", "max-error": 3, "no-deadlock": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("multi.json", document, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"check", "--config-file=multi.json"}, &stdout, &stderr); code != exitCounterexample {
		t.Fatalf("multi-error check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	for _, want := range []string{"Counterexample 1:", "Counterexample 2:", "Counterexample 3:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("multi-error stdout = %q, missing %q", &stdout, want)
		}
	}
}

func TestRunParseHonorsExplicitJSONSourceFormat(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("Spec.input", []byte("---- MODULE ExplicitFormat ----\nEXTENDS Auxiliary\nValue == AuxiliaryValue\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("Auxiliary.tla", []byte("---- MODULE Auxiliary ----\nAuxiliaryValue == TRUE\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("parse.json", []byte(`{
  "source":{"kind":"file","path":"Spec.input","format":"tla"},
  "output":"summary"
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"parse", "--config-file=parse.json"}, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "module ExplicitFormat") {
		t.Fatalf("explicit-format parse = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunCheckLoadsJSONConfigurationWithCLIPrecedence(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("Counter.tla", []byte(`---- MODULE Counter ----
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
	configuration := `{
  "source":"Counter.tla",
  "checker":{"length":2,"inv":["Inv"],"no-deadlock":true},
  "typechecker":{"infer-poly":true}
}`
	if err := os.WriteFile(".apalache.json", []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"check"}, &stdout, &stderr); code != exitCounterexample ||
		!strings.Contains(stdout.String(), "Invariant Inv violated at depth 2") {
		t.Fatalf("configured check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"check", "--length=1"}, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "through depth 1") {
		t.Fatalf("CLI override = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunCheckDebugSnapshotContainsEffectiveCLIValues(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("DebugCounter.tla", []byte(`---- MODULE DebugCounter ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
Inv == x < 5
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := `{
  "source":"DebugCounter.tla",
  "features":[],
  "checker":{"length":3,"inv":[],"no-deadlock":false,"smt-encoding":"funArrays","tuning":{"search.outputTraces":"true"}},
  "typechecker":{"infer-poly":true}
}`
	if err := os.WriteFile("debug.json", []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(directory, "debug-run")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"check", "--config-file=debug.json", "--run-dir=" + runDir, "--debug",
		"--length=1", "--inv=Inv", "--no-deadlock", "--smt-encoding=arrays",
		"--features=rows", "--no-polytypes", "--output-traces=false",
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("debug check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "application-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err = json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	checker, ok := snapshot["checker"].(map[string]any)
	if !ok {
		t.Fatalf("debug checker configuration = %#v", snapshot["checker"])
	}
	if checker["length"] != float64(1) || checker["smt-encoding"] != "arrays" ||
		checker["smt-solver"] != "z3" || checker["algo"] != "incremental" ||
		checker["no-deadlock"] != true || checker["discard-disabled"] != true ||
		checker["init"] != "Init" || checker["next"] != "Next" {
		t.Fatalf("effective checker configuration = %#v", checker)
	}
	if got := checker["inv"].([]any); len(got) != 1 || got[0] != "Inv" {
		t.Fatalf("effective invariants = %#v", checker["inv"])
	}
	tuning := checker["tuning"].(map[string]any)
	if tuning["search.outputTraces"] != "false" {
		t.Fatalf("effective tuning = %#v", tuning)
	}
	typechecker := snapshot["typechecker"].(map[string]any)
	if typechecker["infer-poly"] != false {
		t.Fatalf("effective typechecker configuration = %#v", typechecker)
	}
	features := snapshot["features"].([]any)
	if len(features) != 1 || features[0] != "rows" {
		t.Fatalf("effective features = %#v", features)
	}
}

func TestRunCheckRejectsLegacyExplicitConfiguration(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "--config-file=.apalache.cfg", "Spec.tla"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "legacy Apalache configuration") {
		t.Fatalf("legacy config = %d, stderr = %q", code, &stderr)
	}
}

func TestRunSimulateLoadsJSONConfigurationWithCLIPrecedence(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("Simulation.tla", []byte(`---- MODULE Simulation ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := `{
  "source":"Simulation.tla",
  "checker":{"length":2,"tuning":{"search.simulation":"true","search.simulation.maxRun":"3"}},
  "typechecker":{"infer-poly":true}
}`
	if err := os.WriteFile("simulation.json", []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"simulate", "--config-file=simulation.json"}, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "Simulation completed through depth 2") ||
		!strings.Contains(stdout.String(), "State 2: x = 2") ||
		!strings.Contains(stdout.String(), "Simulation run 3 of 3") {
		t.Fatalf("configured simulation = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"simulate", "--config-file=simulation.json", "--length=1", "--max-run=1"}, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "through depth 1") || strings.Contains(stdout.String(), "State 2:") ||
		strings.Contains(stdout.String(), "Simulation run 2") {
		t.Fatalf("simulation CLI override = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunSimulateDebugSnapshotContainsEffectiveControls(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("DebugSimulation.tla", []byte(`---- MODULE DebugSimulation ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := `{
  "source":"DebugSimulation.tla",
  "checker":{"length":3,"smt-encoding":"funArrays","tuning":{"search.simulation.maxRun":"4","search.outputTraces":"true"}}
}`
	if err := os.WriteFile("simulate-debug.json", []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(directory, "simulate-debug-run")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"simulate", "--config-file=simulate-debug.json", "--run-dir=" + runDir, "--debug",
		"--length=0", "--max-run=2", "--output-traces=false", "--smt-encoding=arrays",
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("debug simulate = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "application-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err = json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	checker := snapshot["checker"].(map[string]any)
	if checker["length"] != float64(0) || checker["smt-encoding"] != "arrays" ||
		checker["init"] != "Init" || checker["next"] != "Next" {
		t.Fatalf("effective simulation checker configuration = %#v", checker)
	}
	tuning := checker["tuning"].(map[string]any)
	if tuning["search.simulation.maxRun"] != "2" || tuning["search.outputTraces"] != "false" {
		t.Fatalf("effective simulation tuning = %#v", tuning)
	}
}

func TestRunSimulateStopsAtConfiguredInvariantErrorBudget(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	source := `---- MODULE SimErrors ----
EXTENDS Integers, FiniteSets
\* @type: Int;
VARIABLE x
Init == x \in {1, 2, 3}
Next == x' = x
Inv == x = 0
View == x
====
`
	document, err := json.Marshal(map[string]any{
		"source": map[string]any{"kind": "string", "content": source, "format": "tla"},
		"checker": map[string]any{
			"length": 0, "inv": []string{"Inv"}, "view": "View", "max-error": 2,
			"tuning": map[string]string{"search.simulation.maxRun": "20"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("sim-errors.json", document, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"simulate", "--config-file=sim-errors.json"}, &stdout, &stderr)
	if code != exitCounterexample || !strings.Contains(stdout.String(), "Simulation counterexample 1:") ||
		!strings.Contains(stdout.String(), "Simulation counterexample 2:") {
		t.Fatalf("invariant simulation = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunTestLoadsJSONConfiguration(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	source := `---- MODULE ConfiguredTest ----
EXTENDS Integers
\* @type: Int;
CONSTANT N
\* @type: Int;
VARIABLE x
CInit == N = 1
Before == x = 0
Increment == x' = x + N
Positive == x > 0
====
`
	configuration, err := json.Marshal(map[string]any{
		"source":  map[string]any{"kind": "string", "content": source, "format": "tla"},
		"checker": map[string]any{"cinit": "CInit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("test.json", configuration, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"test", "--config-file=test.json", "Before", "Increment", "Positive"}
	if code := run(context.Background(), args, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "No example found") {
		t.Fatalf("configured test = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunFrontendCommandsLoadJSONSourceAndOutput(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("Frontend.tla", []byte(`---- MODULE Frontend ----
EXTENDS Integers
\* @type: Int;
CONSTANT N
Value == N + 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		command, output string
	}{
		{"parse", "parsed.json"},
		{"typecheck", "typed.json"},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			configPath := test.command + ".json"
			configuration := `{"source":"Frontend.tla","output":"` + test.output + `","features":[],"typechecker":{"infer-poly":true}}`
			if err := os.WriteFile(configPath, []byte(configuration), 0o600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := run(context.Background(), []string{test.command, "--config-file=" + configPath}, &stdout, &stderr); code != exitOK {
				t.Fatalf("configured %s = %d, stdout = %q, stderr = %q", test.command, code, &stdout, &stderr)
			}
			data, err := os.ReadFile(test.output)
			if err != nil || !bytes.Contains(data, []byte(`"name": "ApalacheIR"`)) {
				t.Fatalf("configured %s output = %v, %v\n%s", test.command, data, err, data)
			}
			if test.command == "typecheck" && !bytes.Contains(data, []byte(`"type": "Int"`)) {
				t.Fatalf("typed output lacks inferred types:\n%s", data)
			}

			stdout.Reset()
			stderr.Reset()
			if code := run(context.Background(), []string{test.command, "--config-file=" + configPath, "--output=summary"}, &stdout, &stderr); code != exitOK ||
				!strings.Contains(stdout.String(), "module Frontend") {
				t.Fatalf("%s CLI output override = %d, stdout = %q, stderr = %q", test.command, code, &stdout, &stderr)
			}
		})
	}
}

func TestRunTranspileLoadsJSONConfiguration(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("ConfiguredVMT.tla", []byte(`---- MODULE ConfiguredVMT ----
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x
Inv == x = x
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := `{
  "source":"ConfiguredVMT.tla",
  "output":"configured.vmt",
  "checker":{"init":"Init","next":"Next","inv":["Inv"]},
  "typechecker":{"infer-poly":true}
}`
	if err := os.WriteFile("transpile.json", []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"transpile", "--config-file=transpile.json"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("configured transpile = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	data, err := os.ReadFile("configured.vmt")
	if err != nil || !bytes.Contains(data, []byte(":action Next$0000")) || !bytes.Contains(data, []byte(":invar-property 0")) {
		t.Fatalf("configured VMT = %v, %v\n%s", data, err, data)
	}

	if err = os.Remove("configured.vmt"); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"transpile", "--config-file=transpile.json", "--output=-"}, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), ":action Next$0000") {
		t.Fatalf("transpile output override = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
	if _, err = os.Stat("configured.vmt"); !os.IsNotExist(err) {
		t.Fatalf("configured output was used despite CLI override: %v", err)
	}
}

func TestRunTraceeLoadsJSONConfiguration(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("TraceConfig.tla", []byte(`---- MODULE TraceConfig ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Double == x + x
Increment == x + 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("input.itf.json", []byte(`{
  "#meta":{"format":"ITF","varTypes":{"x":"Int"}},
  "vars":["x"],"states":[{"x":4}]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := `{
  "source":"TraceConfig.tla",
  "tracee":{"trace":"input.itf.json","expressions":["Double"]},
  "typechecker":{"infer-poly":true}
}`
	if err := os.WriteFile("tracee.json", []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"tracee", "--config-file=tracee.json"}, &stdout, &stderr); code != exitOK ||
		!bytes.Contains(stdout.Bytes(), []byte(`"Double":{"#bigint":"8"}`)) {
		t.Fatalf("configured tracee = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"tracee", "--config-file=tracee.json", "--expressions=Increment"}, &stdout, &stderr); code != exitOK ||
		!bytes.Contains(stdout.Bytes(), []byte(`"Increment":{"#bigint":"5"}`)) || bytes.Contains(stdout.Bytes(), []byte(`"Double"`)) {
		t.Fatalf("tracee expression override = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunTraceeLoadsInMemoryJSONTraceSource(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("TraceMemory.tla", []byte(`---- MODULE TraceMemory ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Double == x + x
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	trace := `{"#meta":{"format":"ITF","varTypes":{"x":"Int"}},"vars":["x"],"states":[{"x":6}]}`
	configuration, err := json.Marshal(map[string]any{
		"source": "TraceMemory.tla",
		"tracee": map[string]any{
			"trace":       map[string]any{"kind": "string", "content": trace, "format": "itf"},
			"expressions": []string{"Double"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("trace-memory.json", configuration, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"tracee", "--config-file=trace-memory.json"}, &stdout, &stderr); code != exitOK ||
		!bytes.Contains(stdout.Bytes(), []byte(`"Double":{"#bigint":"12"}`)) {
		t.Fatalf("in-memory tracee = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}

	x := pal.NewNameExpr("x", pal.Typed(pal.IntT1))
	assignment := pal.MustOperExpr(pal.Op("EQ"), pal.Typed(pal.BoolT1), x,
		pal.NewValueExpr(pal.Int64Value(6), pal.Typed(pal.IntT1)))
	truth := pal.NewValueExpr(pal.BoolValue(true), pal.Typed(pal.BoolT1))
	traceModule := pal.NewModule("LegacyTrace", []pal.Decl{
		pal.NewOperDecl("CInit", nil, truth, pal.Typed(pal.BoolT1)),
		pal.NewOperDecl("State0", nil, assignment, pal.Typed(pal.BoolT1)),
		pal.NewOperDecl("Inv", nil, truth, pal.Typed(pal.BoolT1)),
	})
	legacyTrace, err := pal.MarshalSingleModuleIRJSON(traceModule, pal.IRJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	legacyConfiguration, err := json.Marshal(map[string]any{
		"source": "TraceMemory.tla",
		"tracee": map[string]any{
			"trace":       map[string]any{"kind": "string", "content": string(legacyTrace), "format": "json"},
			"expressions": []string{"Double"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("trace-json.json", legacyConfiguration, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"tracee", "--config-file=trace-json.json"}, &stdout, &stderr); code != exitOK ||
		!bytes.Contains(stdout.Bytes(), []byte(`"Double":{"#bigint":"12"}`)) {
		t.Fatalf("in-memory Apalache JSON tracee = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}
