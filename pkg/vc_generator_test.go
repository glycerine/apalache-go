package apalache

import (
	"errors"
	"testing"
)

func vcOperator(name string, parameters []OperatorParam, body Expr) *OperDecl {
	arguments := make([]Type, len(parameters))
	for index := range arguments {
		arguments[index] = BoolT1
	}
	typ, _ := NewOperatorType(arguments, BoolT1)
	return NewOperDecl(name, parameters, body, Typed(typ))
}

func TestVCGeneratorSplitsStateInvariantAndNegatesFreshCopies(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	a := MustOperExpr(Op("EQ"), Typed(BoolT1), x, x)
	b := MustOperExpr(Op("EQ"), Typed(BoolT1), x, NewValueExpr(Int64Value(1), Typed(IntT1)))
	module := NewModule("M", []Decl{
		NewVarDecl("x", Typed(IntT1)),
		vcOperator("Inv", nil, MustOperExpr(Op("AND"), Typed(BoolT1), a, b)),
	})
	module.AddProperty(PropertyInlined)
	result, err := NewVCGenerator(IdleTracker{}).GenerateInvariant(module, "Inv")
	if err != nil {
		t.Fatal(err)
	}
	for index, original := range []Expr{a, b} {
		positive, found := OperatorBody(result, NormalizedVCInvariantPrefix+string(rune('0'+index)))
		if !found || positive != original {
			t.Fatalf("positive VC %d = %v, found %v", index, positive, found)
		}
		negative, found := OperatorBody(result, NormalizedVCNotInvariantPrefix+string(rune('0'+index)))
		operation, ok := negative.(*OperExpr)
		if !found || !ok || operation.Operator().Name() != "NOT" || operation.Arguments()[0].ID() == original.ID() ||
			!EqualExpr(operation.Arguments()[0], original) {
			t.Fatalf("negative VC %d = %v, found %v", index, negative, found)
		}
	}
	if !result.HasProperty(PropertyInlined) || module.HasProperty(PropertyVCGenerated) {
		t.Fatal("VC generation did not preserve input metadata immutably")
	}
}

func TestVCGeneratorDistributesUniversalsLabelsAndNormalizesAssignment(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	i := NewNameExpr("i", Typed(IntT1))
	assignment := MustOperExpr(Op("Apalache!:="), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), x), x)
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1), i, i)
	conjunction := MustOperExpr(Op("AND"), Typed(BoolT1), assignment, equality)
	labelled := MustOperExpr(Op("LABEL"), Typed(BoolT1), conjunction,
		NewValueExpr(StringValue("L"), Typed(StrT1)))
	quantified := MustOperExpr(Op("FORALL3"), Typed(BoolT1), i,
		NewValueExpr(IntSet, Typed(mustSet(IntT1))), labelled)
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1)), vcOperator("Inv", nil, quantified)})
	result, err := NewVCGenerator(IdleTracker{}).GenerateInvariant(module, "Inv")
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		body, found := OperatorBody(result, NormalizedVCActionPrefix+string(rune('0'+index)))
		if !found {
			t.Fatalf("action VC %d missing", index)
		}
		label := body.(*OperExpr)
		forall := label.Arguments()[0].(*OperExpr)
		leaf := forall.Arguments()[2].(*OperExpr)
		if label.Operator().Name() != "LABEL" || forall.Operator().Name() != "FORALL3" {
			t.Fatalf("VC wrappers = %s / %s", forall.Operator(), label.Operator())
		}
		if index == 0 && leaf.Operator().Name() != "EQ" {
			t.Fatalf("assignment was not normalized: %s", leaf.Operator())
		}
	}
}

func TestVCGeneratorTraceInvariantAndView(t *testing.T) {
	row, _ := NewRowType(map[string]Type{"x": IntT1}, nil)
	history, _ := NewSeqType(NewRecordRowType(row))
	traceType, _ := NewOperatorType([]Type{history}, BoolT1)
	trace := NewOperDecl("TraceInv", []OperatorParam{ValueParam("h")},
		NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(traceType))
	viewType, _ := NewOperatorType(nil, IntT1)
	view := NewOperDecl("View", nil, NewNameExpr("x", Typed(IntT1)), Typed(viewType))
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1)), trace, view})
	generator := NewVCGenerator(IdleTracker{})
	withTrace, err := generator.GenerateInvariant(module, "TraceInv")
	if err != nil {
		t.Fatal(err)
	}
	positive, positiveFound := OperatorBody(withTrace, NormalizedVCTracePrefix+"0")
	negative, negativeFound := OperatorBody(withTrace, NormalizedVCNotTracePrefix+"0")
	if !positiveFound || !negativeFound || positive.ID() == trace.Body().ID() || negative.(*OperExpr).Operator().Name() != "NOT" {
		t.Fatalf("trace VCs = %v/%v, found %v/%v", positive, negative, positiveFound, negativeFound)
	}
	withView, err := generator.GenerateView(withTrace, "View")
	if err != nil {
		t.Fatal(err)
	}
	viewBody, found := OperatorBody(withView, NormalizedVCView)
	if !found || viewBody.ID() == view.Body().ID() || !EqualExpr(viewBody, view.Body()) {
		t.Fatalf("normalized view = %v, found %v", viewBody, found)
	}
}

func TestVCGeneratorRejectsScalaInvalidCases(t *testing.T) {
	temporal := MustOperExpr(Op("GLOBALLY"), Typed(BoolT1), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	module := NewModule("M", []Decl{
		NewConstDecl("NotOperator", Typed(IntT1)),
		vcOperator("Temporal", nil, temporal),
		vcOperator("Parameterized", []OperatorParam{ValueParam("a"), ValueParam("b")}, NewValueExpr(BoolValue(true), Typed(BoolT1))),
	})
	generator := NewVCGenerator(IdleTracker{})
	for _, name := range []string{"Missing", "NotOperator", "Temporal", "Parameterized"} {
		if _, err := generator.GenerateInvariant(module, name); err == nil {
			t.Errorf("invalid invariant %s was accepted", name)
		} else {
			var input *TlaInputError
			if !errors.As(err, &input) {
				t.Errorf("invalid invariant %s error = %T", name, err)
			}
		}
	}
	if _, err := generator.GenerateView(module, "Missing"); err == nil {
		t.Fatal("missing view was accepted")
	}
}
