package apalache

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseApalacheConfigJSONAccumulatesIndependentErrors(t *testing.T) {
	result := ParseApalacheConfigJSON(
		`{"run-dir":1,"debug":"yes","checker":{"length":"ten","inv":[1,true],"smt-solver":"unknown"},"server":{"port":"8822"}}`)
	want := map[string]struct{}{
		"$.run-dir: Expected a JSON string.":                           {},
		"$.debug: Expected a JSON boolean.":                            {},
		"$.checker.inv[0]: Expected a JSON string.":                    {},
		"$.checker.inv[1]: Expected a JSON string.":                    {},
		"$.checker.length: Expected a 32-bit JSON integer.":            {},
		"$.checker.smt-solver: Unexpected SMT solver backend: unknown": {},
		"$.server.port: Expected a 32-bit JSON integer.":               {},
	}
	got := make(map[string]struct{})
	for _, item := range result.Errors() {
		got[item] = struct{}{}
	}
	if result.IsSuccess() || !reflect.DeepEqual(got, want) {
		t.Fatalf("errors = %#v", result.Errors())
	}
}

func TestParseApalacheConfigJSONStrictSchemaAndEnums(t *testing.T) {
	cases := []struct{ input, fragment string }{
		{`checker { smt-solver = cvc5 }`, "Invalid strict JSON"},
		{`{"checker":{},"checker":{}}`, "duplicate key"},
		{`{} {}`, "Trailing content"},
		{`[]`, "Expected the configuration root to be a JSON object"},
		{``, "document is empty"},
		{`{"common":{"debug":true}}`, "$.common: Unknown configuration key."},
		{`{"checker":{"discardDisabled":false}}`, "$.checker.discardDisabled: Unknown configuration key."},
		{`{"checker":{"smt-solver":{"type":"z3"}}}`, "$.checker.smt-solver: Expected a JSON string."},
		{`{"checker":{"smt-encoding":"fun-arrays"}}`, "Unexpected SMT encoding: fun-arrays"},
		{`{"server":{"server-type":"checker-server"}}`, "Unexpected server type: checker-server"},
		{`{"source":{"type":"string","content":"x"}}`, "$.source.type: Unknown configuration key."},
		{`{"source":{"kind":"filesource","path":"Spec.tla"}}`, `Expected "file" or "string", but got "filesource"`},
		{`{"run-dir":"\u0000"}`, "$.run-dir: Invalid path"},
	}
	for _, test := range cases {
		result := ParseApalacheConfigJSON(test.input)
		if result.IsSuccess() || !strings.Contains(strings.Join(result.Errors(), "; "), test.fragment) {
			t.Errorf("%s => %#v, want fragment %q", test.input, result.Errors(), test.fragment)
		}
	}
	valid := ParseApalacheConfigJSON(`{"checker":{"smt-solver":"CVC5","smt-encoding":"funArrays","algo":"REMOTE"},"server":{"server-type":"Explorer"}}`)
	if !valid.IsSuccess() {
		t.Fatal(valid.Errors())
	}
	config := valid.RequireValue()
	if *config.Checker.SMTSolver != SMTSolverCVC5 || *config.Checker.SMTEncoding != SMTEncodingFunArrays ||
		*config.Checker.Algorithm != AlgorithmRemote || *config.Server.ServerType != ServerTypeExplorer {
		t.Fatalf("enums = %#v, %#v", config.Checker, config.Server)
	}
}

func TestApalacheConfigJSONSourceFormsAndRoundTrip(t *testing.T) {
	input := `{
		"command":"check","out-dir":"out","debug":true,"features":["rows"],
		"source":{"kind":"string","content":"---- MODULE M ----\n====","aux":["---- MODULE A ----\n===="],"format":"tla"},
		"output":"result.json","checker":{"tuning":{"b":"2","a":"1"},"no-deadlock":true,"inv":[],"length":7},
		"typechecker":{"infer-poly":false},"tracee":{"trace":{"kind":"file","path":"trace.json","format":"itf"},"expressions":["X"]},
		"server":{"port":9000,"server-type":"checker"}
	}`
	parsed := ParseApalacheConfigJSON(input)
	if !parsed.IsSuccess() {
		t.Fatal(parsed.Errors())
	}
	config := parsed.RequireValue()
	stringSource, ok := config.Source.(StringInputSource)
	if !ok || stringSource.Content != "---- MODULE M ----\n====" || len(stringSource.Auxiliary) != 1 ||
		config.Checker.CheckDeadlocks == nil || *config.Checker.CheckDeadlocks || config.Checker.Invariants == nil ||
		len(*config.Checker.Invariants) != 0 {
		t.Fatalf("decoded config = %#v", config)
	}
	traceFile, ok := config.TraceEvaluation.Trace.(FileInputSource)
	if !ok || traceFile.Format() != InputFormatITF {
		t.Fatalf("trace source = %#v", config.TraceEvaluation.Trace)
	}
	for _, pretty := range []bool{false, true} {
		encoded, err := MarshalApalacheConfigJSON(config, pretty)
		if err != nil || !json.Valid([]byte(encoded)) {
			t.Fatalf("encoded config = %q, %v", encoded, err)
		}
		roundTrip := ParseApalacheConfigJSON(encoded)
		if !roundTrip.IsSuccess() {
			t.Fatal(roundTrip.Errors())
		}
		if !reflect.DeepEqual(roundTrip.RequireValue(), config) {
			t.Fatalf("round trip differs:\n%#v\n%#v", roundTrip.RequireValue(), config)
		}
	}
}

func TestApalacheConfigJSONUsesCompactFileSourceWhenFormatIsInferable(t *testing.T) {
	file, err := NewFileInputSource("Spec.tla")
	if err != nil {
		t.Fatal(err)
	}
	compact, err := MarshalApalacheConfigJSON(ApalacheConfig{Source: file}, false)
	if err != nil || !strings.Contains(compact, `"source":"Spec.tla"`) {
		t.Fatalf("compact file source = %q, %v", compact, err)
	}
	explicit := NewFileInputSourceWithFormat("trace.json", InputFormatITF)
	encoded, err := MarshalApalacheConfigJSON(ApalacheConfig{Source: explicit}, false)
	if err != nil || !strings.Contains(encoded, `"format":"itf"`) || !strings.Contains(encoded, `"kind":"file"`) {
		t.Fatalf("explicit file source = %q, %v", encoded, err)
	}
	roundTrip := ParseApalacheConfigJSON(encoded)
	if !roundTrip.IsSuccess() {
		t.Fatalf("explicit source round trip errors = %#v", roundTrip.Errors())
	}
	if roundTrip.RequireValue().Source.Format() != InputFormatITF {
		t.Fatalf("explicit source round trip = %#v", roundTrip.RequireValue().Source)
	}
}

func TestParseApalacheConfigJSONSupportsLargeSourceStrings(t *testing.T) {
	content := strings.Repeat("x", 20*1000*1000+1)
	encoded, err := json.Marshal(map[string]any{"source": map[string]any{"kind": "string", "content": content}})
	if err != nil {
		t.Fatal(err)
	}
	result := ParseApalacheConfigJSON(string(encoded))
	if !result.IsSuccess() {
		t.Fatal(result.Errors())
	}
	if got := result.RequireValue().Source.(StringInputSource).Content; len(got) != len(content) {
		t.Fatalf("large source length = %d", len(got))
	}
}
