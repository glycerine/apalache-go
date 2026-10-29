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

func TestVCGeneratorScalaOracleCases(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	zero := NewValueExpr(Int64Value(0), Typed(IntT1))
	ten := NewValueExpr(Int64Value(10), Typed(IntT1))
	greaterThanZero := MustOperExpr(Op("GT"), Typed(BoolT1), x, zero)
	lessThanTen := MustOperExpr(Op("LT"), Typed(BoolT1), x, ten)
	assertPair := func(t *testing.T, module *Module, positivePrefix, negativePrefix string, want []Expr) {
		t.Helper()
		result, err := NewVCGenerator(IdleTracker{}).GenerateInvariant(module, "Inv")
		if err != nil {
			t.Fatal(err)
		}
		for index, expected := range want {
			suffix := string(rune('0' + index))
			positive, found := OperatorBody(result, positivePrefix+suffix)
			if !found || !EqualExpr(positive, expected) {
				t.Fatalf("positive VC %d = %v, want %v", index, positive, expected)
			}
			negative, found := OperatorBody(result, negativePrefix+suffix)
			not, ok := negative.(*OperExpr)
			if !found || !ok || not.Operator().Name() != "NOT" || !EqualExpr(not.Arguments()[0], expected) {
				t.Fatalf("negative VC %d = %v, want NOT(%v)", index, negative, expected)
			}
		}
	}
	module := func(body Expr) *Module {
		return NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1)), vcOperator("Inv", nil, body)})
	}

	t.Run("simple invariant", func(t *testing.T) {
		assertPair(t, module(greaterThanZero), NormalizedVCInvariantPrefix, NormalizedVCNotInvariantPrefix,
			[]Expr{greaterThanZero})
	})
	t.Run("action invariant", func(t *testing.T) {
		prime := MustOperExpr(Op("PRIME"), Typed(IntT1), x)
		action := MustOperExpr(Op("GT"), Typed(BoolT1), prime, x)
		assertPair(t, module(action), NormalizedVCActionPrefix, NormalizedVCNotActionPrefix, []Expr{action})
	})
	t.Run("conjunctive invariant", func(t *testing.T) {
		conjunction := MustOperExpr(Op("AND"), Typed(BoolT1), greaterThanZero, lessThanTen)
		assertPair(t, module(conjunction), NormalizedVCInvariantPrefix, NormalizedVCNotInvariantPrefix,
			[]Expr{greaterThanZero, lessThanTen})
	})
	t.Run("conjunction under label", func(t *testing.T) {
		label := NewValueExpr(StringValue("L0"), Typed(StrT1))
		conjunction := MustOperExpr(Op("AND"), Typed(BoolT1), greaterThanZero, lessThanTen)
		body := MustOperExpr(Op("LABEL"), Typed(BoolT1), conjunction, label)
		want := []Expr{
			MustOperExpr(Op("LABEL"), Typed(BoolT1), greaterThanZero, label),
			MustOperExpr(Op("LABEL"), Typed(BoolT1), lessThanTen, label),
		}
		assertPair(t, module(body), NormalizedVCInvariantPrefix, NormalizedVCNotInvariantPrefix, want)
	})
	t.Run("conjunction under nested universals", func(t *testing.T) {
		setType := mustSet(IntT1)
		set := NewNameExpr("S", Typed(setType))
		z := NewNameExpr("z", Typed(IntT1))
		y := NewNameExpr("y", Typed(IntT1))
		gt := MustOperExpr(Op("GT"), Typed(BoolT1), y, zero)
		lt := MustOperExpr(Op("LT"), Typed(BoolT1), y, ten)
		wrap := func(leaf Expr) Expr {
			inner := MustOperExpr(Op("FORALL3"), Typed(BoolT1), y, set, leaf)
			return MustOperExpr(Op("FORALL3"), Typed(BoolT1), z, set, inner)
		}
		body := MustOperExpr(Op("FORALL3"), Typed(BoolT1), z, set,
			MustOperExpr(Op("FORALL3"), Typed(BoolT1), y, set,
				MustOperExpr(Op("AND"), Typed(BoolT1), gt, lt)))
		withSet := NewModule("M", []Decl{
			NewVarDecl("x", Typed(IntT1)), NewVarDecl("S", Typed(setType)), vcOperator("Inv", nil, body),
		})
		assertPair(t, withSet, NormalizedVCInvariantPrefix, NormalizedVCNotInvariantPrefix,
			[]Expr{wrap(gt), wrap(lt)})
	})
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
	h := NewNameExpr("h", Typed(history))
	length := MustOperExpr(Op("Sequences!Len"), Typed(IntT1), h)
	last := MustOperExpr(Op("FUN_APP"), Typed(NewRecordRowType(row)), h, length)
	first := MustOperExpr(Op("FUN_APP"), Typed(NewRecordRowType(row)), h,
		NewValueExpr(Int64Value(1), Typed(IntT1)))
	field := NewValueExpr(StringValue("x"), Typed(StrT1))
	traceBody := MustOperExpr(Op("GT"), Typed(BoolT1),
		MustOperExpr(Op("FUN_APP"), Typed(IntT1), last, field),
		MustOperExpr(Op("FUN_APP"), Typed(IntT1), first, field))
	trace := NewOperDecl("TraceInv", []OperatorParam{ValueParam("h")}, traceBody, Typed(traceType))
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
	if !positiveFound || !negativeFound || positive.ID() == trace.Body().ID() || !EqualExpr(positive, traceBody) ||
		negative.(*OperExpr).Operator().Name() != "NOT" || !EqualExpr(negative.(*OperExpr).Arguments()[0], traceBody) {
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

func TestGenerateVerificationConditionsPassContract(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	inv := vcOperator("Inv", nil, MustOperExpr(Op("EQ"), Typed(BoolT1), x, x))
	viewType, _ := NewOperatorType(nil, IntT1)
	view := NewOperDecl("View", nil, x, Typed(viewType))
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1)), inv, view})
	if _, err := GenerateVerificationConditions(module, DerivedPredicates{Invariants: []string{"Inv"}}, nil); err == nil {
		t.Fatal("VC pass accepted a module that was not inlined")
	}
	module.AddProperty(PropertyInlined)
	result, err := GenerateVerificationConditions(module, DerivedPredicates{
		Invariants: []string{"Inv"}, View: "View",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasProperty(PropertyVCGenerated) || module.HasProperty(PropertyVCGenerated) {
		t.Fatal("VCGenerated property was not applied immutably")
	}
	if _, found := OperatorBody(result, NormalizedVCInvariantPrefix+"0"); !found {
		t.Fatal("configured invariant VC is missing")
	}
	if _, found := OperatorBody(result, NormalizedVCView); !found {
		t.Fatal("configured normalized view is missing")
	}
	state, action, trace, found, err := normalizedCheckerVCs(result)
	if err != nil || !found || len(state) != 1 || len(action) != 0 || len(trace) != 0 ||
		state[0].expression.(*OperExpr).Operator().Name() != "NOT" {
		t.Fatalf("normalized checker VCs = %#v/%#v/%#v, %v, %v", state, action, trace, found, err)
	}

	empty := NewModule("Empty", nil)
	empty.AddProperty(PropertyInlined)
	result, err = GenerateVerificationConditions(empty, DerivedPredicates{}, nil)
	if err != nil || !result.HasProperty(PropertyVCGenerated) || len(result.Declarations()) != 0 {
		t.Fatalf("empty VC pass = %#v, %v", result, err)
	}
}
