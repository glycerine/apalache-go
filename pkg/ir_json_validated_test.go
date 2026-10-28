package apalache

import (
	"errors"
	"testing"
)

func TestValidatedIRJSONDecoderMatchesScalaBuilderCases(t *testing.T) {
	encoder := IRJSONEncoder{}
	decoder := ValidatedIRJSONDecoder{}
	valid := []Expr{
		MustOperExpr(Op("AND"), Typed(BoolT1), NewNameExpr("x", Typed(BoolT1)), NewValueExpr(BoolValue(true), Typed(BoolT1))),
		MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1), NewNameExpr("p", Typed(BoolT1)), intLiteral(1), intLiteral(2)),
		MustOperExpr(Op("LABEL"), Typed(BoolT1), boolExpr(false), NewValueExpr(StringValue("label0"), Typed(StrT1))),
	}
	for _, expression := range valid {
		encoded, err := encoder.Expression(expression)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decoder.Expression(encoded)
		if err != nil || !EqualExpr(decoded, expression) {
			t.Fatalf("validated round trip = %#v, %v", decoded, err)
		}
	}
	invalid := []Expr{
		NewNameExpr("a", Untyped()),
		MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), NewNameExpr("x", Typed(BoolT1))),
		MustOperExpr(Op("PLUS"), Typed(BoolT1), intLiteral(1), intLiteral(2)),
	}
	for _, expression := range invalid {
		encoded, err := encoder.Expression(expression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = decoder.Expression(encoded); err == nil {
			t.Fatalf("accepted invalid typed expression %#v", expression)
		} else {
			var jsonErr *JSONDeserializationError
			if !errors.As(err, &jsonErr) {
				t.Fatalf("error type = %T", err)
			}
		}
	}
}

func TestValidatedIRJSONDecoderDeclarationsModulesAndRoots(t *testing.T) {
	parameterType := mustOperator([]Type{IntT1}, IntT1)
	declaration := NewOperDecl("Inc", []OperatorParam{ValueParam("x")},
		MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)), intLiteral(1)), Typed(parameterType))
	module := NewModule("M", []Decl{declaration})
	root, err := (IRJSONEncoder{}).Root([]*Module{module})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := (ValidatedIRJSONDecoder{}).SingleModule(root)
	if err != nil || len(decoded.Declarations()) != 1 {
		t.Fatalf("validated module = %#v, %v", decoded, err)
	}
	bad := NewOperDecl("Bad", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(BoolT1)), Typed(parameterType))
	badJSON, err := (IRJSONEncoder{}).Declaration(bad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = (ValidatedIRJSONDecoder{}).Declaration(badJSON); err == nil {
		t.Fatal("accepted declaration whose body disagrees with its result type")
	}
}

func TestValidatedIRJSONDecoderValueDependentOperators(t *testing.T) {
	setInt, _ := NewSetType(IntT1)
	row, _ := NewRowType(map[string]Type{"A": IntT1}, nil)
	variant := NewVariantType(row)
	valid := []Expr{
		MustOperExpr(Op("SET_ENUM"), Typed(setInt)),
		MustOperExpr(Op("TUPLE"), Typed(newTupleTypeUnchecked(IntT1, BoolT1)), intLiteral(1), boolExpr(true)),
		MustOperExpr(Op("Variants!Variant"), Typed(variant), NewValueExpr(StringValue("A"), Typed(StrT1)), intLiteral(1)),
		MustOperExpr(Op("Variants!VariantGetUnsafe"), Typed(IntT1), NewValueExpr(StringValue("A"), Typed(StrT1)),
			MustOperExpr(Op("Variants!Variant"), Typed(variant), NewValueExpr(StringValue("A"), Typed(StrT1)), intLiteral(1))),
	}
	decoder := ValidatedIRJSONDecoder{}
	encoder := IRJSONEncoder{}
	for _, expression := range valid {
		encoded, err := encoder.Expression(expression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = decoder.Expression(encoded); err != nil {
			t.Fatalf("rejected %s: %v", expression.(*OperExpr).Operator(), err)
		}
	}
	invalid := []Expr{
		MustOperExpr(Op("REAL_DIV"), Typed(RealT1), intLiteral(1), intLiteral(2)),
		MustOperExpr(Op("Variants!Variant"), Typed(variant), NewValueExpr(StringValue("A"), Typed(StrT1)), boolExpr(true)),
		MustOperExpr(Op("Apalache!MkSeq"), Typed(newSeqTypeUnchecked(IntT1)), NewNameExpr("n", Typed(IntT1)),
			NewNameExpr("f", Typed(mustOperator([]Type{IntT1}, IntT1)))),
	}
	for _, expression := range invalid {
		encoded, err := encoder.Expression(expression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = decoder.Expression(encoded); err == nil {
			t.Fatalf("accepted invalid value-dependent expression %s", expression.(*OperExpr).Operator())
		}
	}
}

func TestValidatedIRJSONDecoderRejectsBinderShadowing(t *testing.T) {
	setInt := newSetTypeUnchecked(IntT1)
	x := func() Expr { return NewNameExpr("x", Typed(IntT1)) }
	domain := MustOperExpr(Op("SET_ENUM"), Typed(setInt), intLiteral(1))
	inner := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x(), domain, boolExpr(true))
	invalid := []Expr{
		MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x(), domain, inner),
		MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x(),
			MustOperExpr(Op("SET_ENUM"), Typed(setInt), x()), boolExpr(true)),
		MustOperExpr(Op("SET_MAP"), Typed(setInt), x(), x(), domain, x(), domain),
	}
	encoder := IRJSONEncoder{}
	decoder := ValidatedIRJSONDecoder{}
	for _, expression := range invalid {
		encoded, err := encoder.Expression(expression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = decoder.Expression(encoded); err == nil {
			t.Fatalf("accepted shadowed binder %s", expression.(*OperExpr).Operator())
		} else {
			var scopeErr *ScopeError
			if !errors.As(err, &scopeErr) {
				t.Fatalf("binder error = %T %v", err, err)
			}
		}
	}
}
