package apalache

import (
	"path/filepath"
	"reflect"
	"testing"
)

func pointer[T any](value T) *T { return &value }

func TestApalacheConfigDefaultsAndFunctionalCommand(t *testing.T) {
	empty := EmptyApalacheConfig()
	defaults := DefaultApalacheConfig()
	merged := empty.MergeWithDefaults()
	if !reflect.DeepEqual(merged, defaults) {
		t.Fatalf("merged defaults differ:\n%#v\n%#v", merged, defaults)
	}
	workingDirectory, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Common.OutDir == nil || *defaults.Common.OutDir != filepath.Join(workingDirectory, "_apalache-out") ||
		defaults.Common.Debug == nil || *defaults.Common.Debug || defaults.Common.Features == nil ||
		len(*defaults.Common.Features) != 0 || *defaults.Checker.Algorithm != AlgorithmIncremental ||
		*defaults.Checker.Length != 10 || *defaults.Checker.MaxError != 1 || *defaults.Checker.SMTSolver != SMTSolverZ3 ||
		*defaults.Typechecker.InferPoly != true || *defaults.Server.Port != 8822 ||
		*defaults.Server.ServerType != ServerTypeChecker {
		t.Fatalf("defaults = %#v", defaults)
	}
	withCommand := empty.WithCommand(ConfigCheck)
	if withCommand.Context.Command == nil || *withCommand.Context.Command != ConfigCheck || empty.Context.Command != nil {
		t.Fatalf("functional command = %#v, original %#v", withCommand.Context, empty.Context)
	}
}

func TestApalacheConfigMergeUsesHigherPrecedenceAndMergesTuning(t *testing.T) {
	highFeatures := []Feature{}
	highInvariants := []string{}
	highTuning := map[string]string{"shared": "high", "high": "yes"}
	lowFeatures := []Feature{RowsFeature}
	lowInvariants := []string{"Inv"}
	lowTuning := map[string]string{"shared": "low", "low": "yes"}
	high := ApalacheConfig{
		Context: RunContextPatch{Command: pointer(ConfigCheck)},
		Common:  CommonPatch{Debug: pointer(false), Features: &highFeatures},
		Checker: CheckerPatch{Tuning: &highTuning, Invariants: &highInvariants, Length: pointer(4)},
		Output:  pointer("high.json"),
	}
	lowSource := NewStringInputSource("---- MODULE M ----\n====", nil, InputFormatTLA)
	low := ApalacheConfig{
		Context: RunContextPatch{Command: pointer(ConfigServer), ConfigFile: pointer("low.json")},
		Common:  CommonPatch{Debug: pointer(true), Profiling: pointer(true), Features: &lowFeatures},
		Source:  lowSource, Output: pointer("low.json"),
		Checker: CheckerPatch{Tuning: &lowTuning, Invariants: &lowInvariants, MaxError: pointer(2)},
	}
	merged := high.MergeWithLower(low)
	if *merged.Context.Command != ConfigCheck || *merged.Context.ConfigFile != "low.json" || *merged.Common.Debug ||
		!*merged.Common.Profiling || len(*merged.Common.Features) != 0 || len(*merged.Checker.Invariants) != 0 ||
		*merged.Output != "high.json" || merged.Source.String() != lowSource.String() || *merged.Checker.Length != 4 ||
		*merged.Checker.MaxError != 2 || !reflect.DeepEqual(*merged.Checker.Tuning,
		map[string]string{"shared": "high", "high": "yes", "low": "yes"}) {
		t.Fatalf("merged config = %#v", merged)
	}
	(*merged.Checker.Tuning)["shared"] = "changed"
	*merged.Common.Features = append(*merged.Common.Features, RowsFeature)
	if highTuning["shared"] != "high" || lowTuning["shared"] != "low" || len(highFeatures) != 0 {
		t.Fatal("merged config shares mutable patch storage")
	}
}

func TestConfigPatchClonesEveryMutableVariant(t *testing.T) {
	features := []Feature{RowsFeature}
	invariants := []string{"Inv"}
	expressions := []string{"X"}
	tuning := map[string]string{"k": "v"}
	patches := []ConfigPatch{
		RunContextPatch{}, CommonPatch{Features: &features}, CheckerPatch{Tuning: &tuning, Invariants: &invariants},
		TypecheckerPatch{}, TraceEvaluationPatch{Expressions: &expressions}, ServerPatch{},
	}
	if len(patches) != 6 {
		t.Fatalf("patch variants = %d", len(patches))
	}
	common := patches[1].(CommonPatch).Clone()
	checker := patches[2].(CheckerPatch).Clone()
	trace := patches[4].(TraceEvaluationPatch).Clone()
	(*common.Features)[0] = ImpreciseRecordsFeature
	(*checker.Tuning)["k"] = "changed"
	(*checker.Invariants)[0] = "changed"
	(*trace.Expressions)[0] = "changed"
	if features[0] != RowsFeature || tuning["k"] != "v" || invariants[0] != "Inv" || expressions[0] != "X" {
		t.Fatal("patch clone exposed mutable inputs")
	}
}
