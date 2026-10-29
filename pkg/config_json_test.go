package apalache

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
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
		{`{"checker":{"smt-encoding":"oopsla-19"}}`, "Unexpected SMT encoding: oopsla-19"},
		{`{"server":{"server-type":"checker-server"}}`, "Unexpected server type: checker-server"},
		{`{"server":{"server-type":"explorer-server"}}`, "Unexpected server type: explorer-server"},
		{`{"source":{"type":"string","content":"x"}}`, "$.source.type: Unknown configuration key."},
		{`{"source":{"kind":"file","file":"Spec.tla"}}`, "$.source.file: Unknown configuration key."},
		{`{"source":{"kind":"filesource","path":"Spec.tla"}}`, `Expected "file" or "string", but got "filesource"`},
		{`{"source":{"kind":"stringsource","content":"x"}}`, `Expected "file" or "string", but got "stringsource"`},
		{`{"input":{"source":"Spec.tla"}}`, "$.input: Unknown configuration key."},
		{`{"output":{"output":"output.tla"}}`, "$.output: Expected a JSON string."},
		{`{"checker":{"timeout-smt-sec":12}}`, "$.checker.timeout-smt-sec: Unknown configuration key."},
		{`{"checker":{"no-deadlocks":true}}`, "$.checker.no-deadlocks: Unknown configuration key."},
		{`{"checker":{"temporal-props":["P"]}}`, "$.checker.temporal-props: Unknown configuration key."},
		{`{"typechecker":{"inferpoly":true}}`, "$.typechecker.inferpoly: Unknown configuration key."},
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

func TestApalacheConfigJSONWritesCanonicalTopLevelAndScalarForms(t *testing.T) {
	output := "output.tla"
	config := ApalacheConfig{
		Common:  CommonPatch{Debug: pointer(true)},
		Source:  NewFileInputSourceWithFormat("Spec.tla", InputFormatTLA),
		Output:  &output,
		Checker: CheckerPatch{SMTSolver: pointer(SMTSolverCVC5)},
	}
	encoded, err := MarshalApalacheConfigJSON(config, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"source":"Spec.tla"`, `"output":"output.tla"`, `"debug":true`, `"smt-solver":"cvc5"`} {
		if !strings.Contains(encoded, fragment) {
			t.Errorf("canonical JSON %s lacks %s", encoded, fragment)
		}
	}
	for _, obsolete := range []string{`"input"`, `"common"`} {
		if strings.Contains(encoded, obsolete) {
			t.Errorf("canonical JSON %s contains obsolete section %s", encoded, obsolete)
		}
	}
}

func TestApalacheConfigJSONMergeTuningPerKey(t *testing.T) {
	lower := ParseApalacheConfigJSON(
		`{"source":"Lower.tla","output":"lower.json","checker":{"tuning":{"a":"1","shared":"low"},"smt-solver":"z3"}}`).RequireValue()
	higher := ParseApalacheConfigJSON(
		`{"source":"Higher.tla","checker":{"tuning":{"b":"2","shared":"high"},"smt-solver":"cvc5"}}`).RequireValue()
	merged := higher.MergeWithLower(lower)
	wantTuning := map[string]string{"a": "1", "b": "2", "shared": "high"}
	if merged.Source.String() != "Higher.tla" || merged.Output == nil || *merged.Output != "lower.json" ||
		merged.Checker.Tuning == nil || !reflect.DeepEqual(*merged.Checker.Tuning, wantTuning) ||
		merged.Checker.SMTSolver == nil || *merged.Checker.SMTSolver != SMTSolverCVC5 {
		t.Fatalf("merged configuration = %#v", merged)
	}
}

func TestApalacheConfigJSONManualExamples(t *testing.T) {
	data, err := os.ReadFile("../docs/src/apalache/config.md")
	if err != nil {
		t.Fatal(err)
	}
	examples := regexp.MustCompile("(?s)```json\\s*(.*?)\\s*```").FindAllSubmatch(data, -1)
	if len(examples) == 0 {
		t.Fatal("configuration manual contains no JSON examples")
	}
	for index, example := range examples {
		result := ParseApalacheConfigJSON(string(example[1]))
		if !result.IsSuccess() {
			t.Errorf("manual JSON example %d: %#v", index, result.Errors())
		}
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
