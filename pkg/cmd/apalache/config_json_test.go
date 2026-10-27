package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
  "checker":{"length":2},
  "typechecker":{"infer-poly":true}
}`
	if err := os.WriteFile("simulation.json", []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"simulate", "--config-file=simulation.json"}, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "Simulation completed through depth 2") ||
		!strings.Contains(stdout.String(), "State 2: x = 2") {
		t.Fatalf("configured simulation = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"simulate", "--config-file=simulation.json", "--length=1"}, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "through depth 1") || strings.Contains(stdout.String(), "State 2:") {
		t.Fatalf("simulation CLI override = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
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
