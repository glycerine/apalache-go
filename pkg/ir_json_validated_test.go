package apalache

import (
	"context"
	"errors"
	"os"
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
	badTheorem := NewTheoremDecl("BadTheorem", boolExpr(true), Typed(IntT1))
	badJSON, err = (IRJSONEncoder{}).Declaration(badTheorem)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = (ValidatedIRJSONDecoder{}).Declaration(badJSON); err == nil {
		t.Fatal("accepted theorem whose body disagrees with its declaration type")
	}
}

func TestValidatedIRJSONDecoderValueDependentOperators(t *testing.T) {
	setInt, _ := NewSetType(IntT1)
	row, _ := NewRowType(map[string]Type{"A": IntT1}, nil)
	variant := NewVariantType(row)
	variable, _ := NewVarType(0)
	variableRow, _ := NewRowType(map[string]Type{"A": variable}, nil)
	variableVariant := NewVariantType(variableRow)
	variableVariantValue := MustOperExpr(Op("Variants!Variant"), Typed(variableVariant),
		NewValueExpr(StringValue("A"), Typed(StrT1)), intLiteral(1))
	setVariant := newSetTypeUnchecked(variant)
	variantValue := MustOperExpr(Op("Variants!Variant"), Typed(variant),
		NewValueExpr(StringValue("A"), Typed(StrT1)), intLiteral(1))
	variantSet := MustOperExpr(Op("SET_ENUM"), Typed(setVariant), variantValue)
	sparse, _ := NewSparseTupleType(map[int]Type{3: StrT1})
	record, _ := NewRecordType(map[string]Type{"items": newSeqTypeUnchecked(IntT1)})
	recordValue := NewNameExpr("record", Typed(record))
	updatePath := MustOperExpr(Op("TUPLE"), Typed(newTupleTypeUnchecked(StrT1, IntT1)),
		NewValueExpr(StringValue("items"), Typed(StrT1)), intLiteral(2))
	valid := []Expr{
		MustOperExpr(Op("SET_ENUM"), Typed(setInt)),
		MustOperExpr(Op("TUPLE"), Typed(newTupleTypeUnchecked(IntT1, BoolT1)), intLiteral(1), boolExpr(true)),
		variantValue,
		MustOperExpr(Op("Variants!VariantFilter"), Typed(setInt),
			NewValueExpr(StringValue("A"), Typed(StrT1)), variantSet),
		MustOperExpr(Op("Variants!VariantGetOrElse"), Typed(IntT1),
			NewValueExpr(StringValue("A"), Typed(StrT1)), variantValue, intLiteral(0)),
		MustOperExpr(Op("Variants!VariantGetUnsafe"), Typed(IntT1), NewValueExpr(StringValue("A"), Typed(StrT1)),
			variantValue),
		MustOperExpr(Op("FUN_APP"), Typed(StrT1), NewNameExpr("sparse", Typed(sparse)), intLiteral(3)),
		MustOperExpr(Op("DOMAIN"), Typed(setInt), NewNameExpr("sparseDomain", Typed(sparse))),
		MustOperExpr(Op("EXCEPT"), Typed(record), recordValue, updatePath, intLiteral(9)),
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
		MustOperExpr(Op("Variants!VariantFilter"), Typed(setVariant),
			NewValueExpr(StringValue("A"), Typed(StrT1)), variantSet),
		MustOperExpr(Op("Variants!VariantFilter"), Typed(setInt),
			NewValueExpr(StringValue("A"), Typed(StrT1)), variantValue),
		MustOperExpr(Op("Variants!VariantGetOrElse"), Typed(variable),
			NewValueExpr(StringValue("A"), Typed(StrT1)), variableVariantValue, intLiteral(0)),
		MustOperExpr(Op("FUN_APP"), Typed(StrT1), NewNameExpr("dynamicSparse", Typed(sparse)),
			NewNameExpr("index", Typed(IntT1))),
		MustOperExpr(Op("EXCEPT"), Typed(record), recordValue, updatePath, boolExpr(false)),
		MustOperExpr(Op("EXCEPT"), Typed(record), recordValue,
			NewNameExpr("path", Typed(newTupleTypeUnchecked(StrT1))), intLiteral(9)),
		MustOperExpr(Op("Apalache!MkSeq"), Typed(newSeqTypeUnchecked(IntT1)), NewNameExpr("n", Typed(IntT1)),
			NewNameExpr("f", Typed(mustOperator([]Type{IntT1}, IntT1)))),
	}
	decimal, err := NewDecimalValue("1.5")
	if err != nil {
		t.Fatal(err)
	}
	invalid = append(invalid, NewValueExpr(decimal, Typed(RealT1)))
	for _, name := range []string{
		"SequencesExt!ToSet", "TLC!ToString", "ApalacheInternal!__ApalacheSeqCapacity",
		"ApalacheInternal!__CompactFunSetExpand",
	} {
		invalid = append(invalid, MustOperExpr(Op(name), Typed(IntT1), intLiteral(1)))
	}
	for _, expression := range invalid {
		encoded, err := encoder.Expression(expression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = decoder.Expression(encoded); err == nil {
			t.Fatalf("accepted invalid value-dependent expression %T", expression)
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

func TestValidatedIRJSONDecoderAcceptsTypedTlagoOracleCorpus(t *testing.T) {
	files := []string{
		"VariantTrace.tla", "DeterministicDeadlock.tla", "ConfiguredConstraints.tla",
		"RowRecordTrace.tla", "StructuredTrace.tla",
	}
	for _, filename := range files {
		t.Run(filename, func(t *testing.T) {
			source, err := os.ReadFile("testdata/oracle/" + filename)
			if err != nil {
				t.Fatal(err)
			}
			frontend := ParseAndTypeCheckSource(context.Background(), filename, string(source),
				FrontendOptions{IncludeSource: true}, TypeCheckOptions{UseRows: true})
			if !frontend.OK() {
				t.Fatalf("frontend diagnostics: %+v", frontend.Diagnostics)
			}
			typedJSON, err := MarshalSingleModuleIRJSON(frontend.Native, IRJSONOptions{SourceStore: frontend.Sources})
			if err != nil {
				t.Fatal(err)
			}
			representation, err := ParseJSONRepresentation(typedJSON)
			if err != nil {
				t.Fatal(err)
			}
			module, err := (ValidatedIRJSONDecoder{SourceStore: NewSourceStore()}).SingleModule(representation)
			if err != nil {
				t.Fatalf("%v: %v", err, errors.Unwrap(err))
			}
			if module.Name() == "" || len(module.Declarations()) == 0 {
				t.Fatalf("decoded module = %#v", module)
			}
		})
	}
}
