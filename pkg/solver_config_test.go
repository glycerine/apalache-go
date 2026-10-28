package apalache

import (
	"context"
	"reflect"
	"testing"
)

func TestSolverConfigDefaultsAndEnumParsingMatchScala(t *testing.T) {
	config := DefaultSolverConfig()
	if config.Debug || config.Profile || config.RandomSeed != 0 || config.SMTEncoding != SMTEncodingOOPSLA19 ||
		config.Z3StatsSeconds != 60 || config.SMTSolver != SMTSolverZ3 || len(config.SolverParameters()) != 0 {
		t.Fatalf("default solver config = %#v", config)
	}
	for _, encoding := range []SMTEncoding{SMTEncodingOOPSLA19, SMTEncodingArrays, SMTEncodingFunArrays} {
		parsed, err := ParseSMTEncoding(string(encoding))
		if err != nil || parsed != encoding {
			t.Fatalf("encoding %s => %s, %v", encoding, parsed, err)
		}
	}
	if _, err := ParseSMTEncoding("OOPSLA19"); err == nil {
		t.Fatal("case-changed encoding was accepted")
	}
	for input, want := range map[string]SMTSolverBackend{"z3": SMTSolverZ3, "Z3": SMTSolverZ3, "cVc5": SMTSolverCVC5} {
		parsed, err := ParseSMTSolverBackend(input)
		if err != nil || parsed != want {
			t.Fatalf("solver %s => %s, %v", input, parsed, err)
		}
	}
}

func TestSolverConfigCopiesParameters(t *testing.T) {
	parameters := map[string]any{"smt.mbqi": true}
	config := NewSolverConfig(true, true, 17, SMTEncodingOOPSLA19, 5, SMTSolverZ3, parameters)
	parameters["smt.mbqi"] = false
	if !reflect.DeepEqual(config.SolverParameters(), map[string]any{"smt.mbqi": true}) {
		t.Fatalf("parameters = %#v", config.SolverParameters())
	}
	copy := config.SolverParameters()
	copy["smt.mbqi"] = false
	if config.SolverParameters()["smt.mbqi"] != true {
		t.Fatal("SolverParameters exposed mutable state")
	}
	updated := config.WithSolverParameters(map[string]any{"sat.phase": "random"})
	if len(config.SolverParameters()) != 1 || updated.SolverParameters()["sat.phase"] != "random" {
		t.Fatalf("functional parameter update: old=%v new=%v", config.SolverParameters(), updated.SolverParameters())
	}
}

func TestSolverConfigConstructsPinnedG3WithTypedParameters(t *testing.T) {
	config := NewSolverConfig(false, false, 17, SMTEncodingOOPSLA19, 60, SMTSolverZ3, map[string]any{
		"z3.smt.mbqi": true, "sat.random_freq": 0.25, "pp.max_width": 120,
	})
	solver, err := NewG3SolverContextWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	truth, err := solver.Bool(true)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(truth); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("configured g3 = %v, %v", status, checkErr)
	}
}

func TestSolverConfigConstructsPinnedG3ForEveryEncoding(t *testing.T) {
	for _, encoding := range []SMTEncoding{SMTEncodingOOPSLA19, SMTEncodingArrays, SMTEncodingFunArrays} {
		t.Run(string(encoding), func(t *testing.T) {
			config := NewSolverConfig(false, false, 0, encoding, 60, SMTSolverZ3, nil)
			solver, err := NewG3SolverContextWithConfig(config)
			if err != nil {
				t.Fatal(err)
			}
			defer solver.Close()
		})
	}
}

func TestSolverConfigRejectsUnavailableOrInvalidBackends(t *testing.T) {
	tests := []SolverConfig{
		NewSolverConfig(false, false, 0, SMTEncoding("unknown"), 60, SMTSolverZ3, nil),
		NewSolverConfig(false, false, 0, SMTEncodingOOPSLA19, 60, SMTSolverCVC5, nil),
		NewSolverConfig(false, false, -1, SMTEncodingOOPSLA19, 60, SMTSolverZ3, nil),
		NewSolverConfig(false, false, 0, SMTEncodingOOPSLA19, 60, SMTSolverZ3, map[string]any{"x": []int{1}}),
	}
	for _, config := range tests {
		if solver, err := NewG3SolverContextWithConfig(config); err == nil {
			solver.Close()
			t.Fatalf("invalid config succeeded: %#v", config)
		}
	}
}
