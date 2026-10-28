package apalache

import (
	"reflect"
	"testing"
)

func TestConfigParseResultSuccessFailureWarningsAndCopies(t *testing.T) {
	warnings := []string{"first"}
	success := ConfigSuccess(42, warnings...)
	warnings[0] = "changed"
	if !success.IsSuccess() || success.RequireValue() != 42 || !reflect.DeepEqual(success.Warnings(), []string{"first"}) {
		t.Fatalf("success = %#v", success)
	}
	returnedWarnings := success.Warnings()
	returnedWarnings[0] = "changed"
	if success.Warnings()[0] != "first" {
		t.Fatal("warnings exposed shared storage")
	}
	failure := ConfigFailure[int]([]string{"bad one", "bad two"}, "warning")
	if failure.IsSuccess() || !reflect.DeepEqual(failure.Errors(), []string{"bad one", "bad two"}) {
		t.Fatalf("failure = %#v", failure)
	}
	converted := ConfigFailureFrom[string](failure)
	if !reflect.DeepEqual(converted.Errors(), failure.Errors()) || !reflect.DeepEqual(converted.Warnings(), failure.Warnings()) {
		t.Fatalf("converted failure = %#v", converted)
	}
	withWarnings := ConfigWithWarnings(success, "second")
	if !reflect.DeepEqual(withWarnings.Warnings(), []string{"first", "second"}) ||
		!reflect.DeepEqual(success.Warnings(), []string{"first"}) {
		t.Fatalf("warning append = %#v, original %#v", withWarnings.Warnings(), success.Warnings())
	}
	message := ConfigFailureMessage[int]("single")
	if !reflect.DeepEqual(message.Errors(), []string{"single"}) {
		t.Fatalf("single failure = %#v", message.Errors())
	}
}

func TestConfigParseResultRequireValuePanicsOnContractViolation(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != "Configuration parse result has no value" {
			t.Fatalf("panic = %#v", recovered)
		}
	}()
	ConfigFailureMessage[int]("bad").RequireValue()
}

func TestValidatedOptionGraphClonesMutableInputs(t *testing.T) {
	runDir, output, cinit, view := "run", "out.json", "CInit", "View"
	deadlock := true
	trace := ValidatedTraceOptions{
		Common:  CommonOptions{Features: []Feature{RowsFeature}, RunDir: &runDir},
		Output:  &output,
		Checker: CheckerOptions{Length: 10, Tuning: map[string]string{"search": "bfs"}},
		Specification: SpecificationOptions{
			BehaviorSpec: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInitializer: &cinit,
			Invariants: []string{"Inv"}, TemporalProperties: []string{"Live"}, View: &view,
			Persistent: []string{"Init"}, TLCConfig: &TLCConfigInput{Path: "M.cfg", Config: TLCConfig{
				Invariants: []string{"CfgInv"}, CheckDeadlock: &deadlock,
			}},
		},
		TraceEvaluation: TraceEvaluationOptions{
			Trace: NewStringInputSource(`{"vars":[]}`, nil, InputFormatITF), Expressions: []string{"X"},
		},
	}
	clone := trace.Clone()
	clone.Common.Features[0] = ImpreciseRecordsFeature
	*clone.Common.RunDir = "changed"
	*clone.Output = "changed"
	clone.Checker.Tuning["search"] = "dfs"
	clone.Specification.Invariants[0] = "changed"
	*clone.Specification.TLCConfig.Config.CheckDeadlock = false
	clone.TraceEvaluation.Expressions[0] = "changed"
	if trace.Common.Features[0] != RowsFeature || *trace.Common.RunDir != "run" || *trace.Output != "out.json" ||
		trace.Checker.Tuning["search"] != "bfs" || trace.Specification.Invariants[0] != "Inv" ||
		!*trace.Specification.TLCConfig.Config.CheckDeadlock || trace.TraceEvaluation.Expressions[0] != "X" {
		t.Fatalf("clone mutated original: %#v", trace)
	}
	shorter := trace.WithLength(3)
	if shorter.Checker.Length != 3 || trace.Checker.Length != 10 {
		t.Fatalf("functional length update = %d, original %d", shorter.Checker.Length, trace.Checker.Length)
	}
}

func TestEveryValidatedOptionsShapeImplementsMarker(t *testing.T) {
	values := []ValidatedOptions{
		CommonOptions{}, CommandInitializationOptions{}, ModuleIOOptions{}, TypecheckerOptions{}, CheckerOptions{},
		TLCConfigInput{}, SpecificationOptions{}, TraceEvaluationOptions{}, ServerOptions{}, ValidatedParseOptions{},
		ValidatedTypecheckOptions{}, ValidatedCheckOptions{}, ValidatedTraceOptions{}, ValidatedServerOptions{},
	}
	if len(values) != 14 {
		t.Fatalf("validated option variants = %d", len(values))
	}
}
