package apalache

import (
	"context"
	"testing"
)

func TestEvaluateTraceOverTypedFrontendModule(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "TraceEval.tla", `---- MODULE TraceEval ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Square == x * x
Class == IF x % 2 = 0 THEN "even" ELSE "odd"
\* @type: () => Seq(Int);
Pair == <<x, x + 1>>
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	states := []TraceState{
		{"x": intLiteral(1)},
		{"x": intLiteral(2)},
		{"x": intLiteral(3)},
	}
	evaluation, err := EvaluateTrace(context.Background(), frontend.Native, states, []string{"Square", "Class", "Pair"})
	if err != nil {
		t.Fatal(err)
	}
	if len(evaluation.States) != 3 || len(evaluation.Module.VarDeclarations()) != 3 {
		t.Fatalf("evaluation shape = %d states, %d variables", len(evaluation.States), len(evaluation.Module.VarDeclarations()))
	}
	want := []map[string]string{
		{"Square": "1", "Class": `"odd"`, "Pair": "<<1, 2>>"},
		{"Square": "4", "Class": `"even"`, "Pair": "<<2, 3>>"},
		{"Square": "9", "Class": `"odd"`, "Pair": "<<3, 4>>"},
	}
	for index, state := range evaluation.States {
		for name, expected := range want[index] {
			actual, formatErr := FormatTLAExpression(state[name])
			if formatErr != nil || actual != expected {
				t.Errorf("state %d %s = %q, %v; want %q", index, name, actual, formatErr, expected)
			}
		}
	}
	outputStates := append([]TraceState{{}}, evaluation.States...)
	if _, err = MarshalITFTrace(evaluation.Module, outputStates, ITFOptions{}); err != nil {
		t.Fatalf("marshal evaluated trace: %v", err)
	}
}

func TestEvaluateTraceRejectsUndefinedParameterizedAndMissingValues(t *testing.T) {
	module := NewModule("M", []Decl{
		NewVarDecl("x", Typed(IntT1)),
		NewOperDecl("F", []OperatorParam{ValueParam("a")}, NewNameExpr("a", Typed(IntT1)), Typed(mustOperator([]Type{IntT1}, IntT1))),
		NewOperDecl("X", nil, NewNameExpr("x", Typed(IntT1)), Typed(mustOperator(nil, IntT1))),
	})
	module.AddProperty(PropertyTypeChecked)
	if _, err := EvaluateTrace(context.Background(), module, []TraceState{{"x": intLiteral(1)}}, []string{"Missing"}); err == nil {
		t.Fatal("undefined expression was accepted")
	}
	if _, err := EvaluateTrace(context.Background(), module, []TraceState{{"x": intLiteral(1)}}, []string{"F"}); err == nil {
		t.Fatal("parameterized expression was accepted")
	}
	if _, err := EvaluateTrace(context.Background(), module, []TraceState{{}}, []string{"X"}); err == nil {
		t.Fatal("missing trace variable was accepted")
	}
}
