package apalache

import (
	"fmt"
	"testing"
)

func requireExprType(t *testing.T, expr Expr, want Type) {
	t.Helper()
	got, err := ExpressionType(expr)
	if err != nil || !EqualTypes(got, want) {
		t.Fatalf("expression type = %v, %v; want %v", got, err, want)
	}
}

func TestTypedBuilderLiteralsAndSignatureApplication(t *testing.T) {
	builder := NewTypedBuilder()
	one := builder.Int64(1)
	two := builder.Int64(2)
	plus, err := builder.Apply(Op("PLUS"), one, two)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, plus, IntT1)
	comparison, err := builder.Apply(Op("LT"), plus, two)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, comparison, BoolT1)
	if _, err := builder.Apply(Op("PLUS"), one, builder.Bool(true)); err == nil {
		t.Fatal("typed builder accepted invalid arithmetic")
	}
	if _, err := builder.Apply(Op("NOT"), NewNameExpr("untyped", Untyped())); err == nil {
		t.Fatal("typed builder accepted an untyped child")
	}
}

func TestTypedBuilderStringsAndModelValues(t *testing.T) {
	builder := NewTypedBuilder()
	if _, err := builder.String("1_OF_PERSON"); err == nil {
		t.Fatal("model value accepted as an ordinary string")
	}
	person := mustNamedConst(t, "PERSON")
	value, err := builder.ModelValue("1", person)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, value, person)
	parsed, err := builder.ParsedModelValue("alice_OF_PERSON")
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, parsed, person)
	if _, err := builder.ParsedModelValue("alice"); err == nil {
		t.Fatal("ordinary string accepted as a parsed model value")
	}
	requireExprType(t, builder.PredefinedSet(NatSet), mustSetType(t, IntT1))
}

func TestTypedBuilderTuplesAndSequences(t *testing.T) {
	builder := NewTypedBuilder()
	one, truth := builder.Int64(1), builder.Bool(true)
	tuple, err := builder.Tuple(one, truth)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, tuple, mustTupleType(t, IntT1, BoolT1))
	sequence, err := builder.Sequence(one, builder.Int64(2))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, sequence, mustSeqType(t, IntT1))
	if _, err := builder.Sequence(one, truth); err == nil {
		t.Fatal("heterogeneous sequence accepted")
	}
	requireExprType(t, builder.EmptySequence(StrT1), mustSeqType(t, StrT1))
	requireExprType(t, builder.EmptySet(BoolT1), mustSetType(t, BoolT1))
}

func TestTypedBuilderRecordsAndRecordSets(t *testing.T) {
	builder := NewTypedBuilder()
	record, err := builder.Record(
		RecordField{Name: "count", Value: builder.Int64(1)},
		RecordField{Name: "ready", Value: builder.Bool(true)},
	)
	if err != nil {
		t.Fatal(err)
	}
	wantRecord, _ := NewRecordType(map[string]Type{"count": IntT1, "ready": BoolT1})
	requireExprType(t, record, wantRecord)
	if _, err := builder.Record(
		RecordField{Name: "same", Value: builder.Int64(1)},
		RecordField{Name: "same", Value: builder.Int64(2)},
	); err == nil {
		t.Fatal("duplicate record fields accepted")
	}

	tail := mustNumberedVar(t, 4)
	rowRecord, err := builder.RowRecord(&tail, RecordField{Name: "count", Value: builder.Int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	wantRow := NewRecordRowType(mustRowType(t, map[string]Type{"count": IntT1}, &tail))
	requireExprType(t, rowRecord, wantRow)

	intSet := builder.PredefinedSet(IntSet)
	boolSet := builder.PredefinedSet(BoolSet)
	recordSet, err := builder.RecordSet(
		RecordField{Name: "count", Value: intSet}, RecordField{Name: "ready", Value: boolSet})
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, recordSet, mustSetType(t, wantRecord))
}

func TestTypedBuilderRowRecordScalaOracleCases(t *testing.T) {
	builder := NewTypedBuilder()
	for index := range 1_000 {
		fieldCount := index%4 + 1
		fields := make([]RecordField, fieldCount)
		for fieldIndex := range fields {
			value := Expr(builder.Int64(int64(fieldIndex)))
			if fieldIndex%2 == 1 {
				value = builder.Bool(fieldIndex%4 == 1)
			}
			fields[fieldIndex] = RecordField{Name: fmt.Sprintf("f%d", fieldIndex), Value: value}
		}
		var tail *VarType
		if index%2 == 1 {
			value := mustNumberedVar(t, index%26)
			tail = &value
		}
		record, err := builder.RowRecord(tail, fields...)
		if err != nil || record.Operator().Name() != "RECORD" {
			t.Fatalf("row record %d = %v, %v", index, record, err)
		}
	}

	a, err := builder.RowRecord(nil, RecordField{Name: "f1", Value: builder.Int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	stringValue, err := builder.String("s")
	if err != nil {
		t.Fatal(err)
	}
	b, err := builder.RowRecord(nil,
		RecordField{Name: "f1", Value: builder.Int64(2)}, RecordField{Name: "f2", Value: stringValue})
	if err != nil {
		t.Fatal(err)
	}
	tail := mustNumberedVar(t, 1)
	openRow := NewRecordRowType(mustRowType(t, map[string]Type{"f1": IntT1}, &tail))
	operatorType := mustOperatorType(t, []Type{openRow}, IntT1)
	operator := builder.Name("op", operatorType)
	first, err := builder.Apply(Op("OPER_APP"), operator, a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := builder.Apply(Op("OPER_APP"), operator, b)
	if err != nil {
		t.Fatal(err)
	}
	applications, err := builder.Sequence(first, second)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, applications, mustSeqType(t, IntT1))
}

func TestTypedBuilderApplicatives(t *testing.T) {
	builder := NewTypedBuilder()
	functionType := mustFunType(t, IntT1, BoolT1)
	function := builder.Name("f", functionType)
	application, err := builder.FunctionApplication(function, builder.Int64(1))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, application, BoolT1)
	domain, err := builder.Domain(function)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, domain, mustSetType(t, IntT1))
	updated, err := builder.Except(function, builder.Int64(1), builder.Bool(false))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, updated, functionType)

	tuple, _ := builder.Tuple(builder.Int64(1), builder.Bool(true))
	second, err := builder.FunctionApplication(tuple, builder.Int64(2))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, second, BoolT1)
	if _, err := builder.FunctionApplication(tuple, builder.Int64(3)); err == nil {
		t.Fatal("out-of-range tuple application accepted")
	}

	record, _ := builder.Record(RecordField{Name: "field", Value: builder.Int64(1)})
	fieldName, _ := builder.String("field")
	field, err := builder.FunctionApplication(record, fieldName)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, field, IntT1)
}

func TestTypedBuilderVariants(t *testing.T) {
	builder := NewTypedBuilder()
	variantType := NewVariantType(mustRowType(t, map[string]Type{"Number": IntT1, "Flag": BoolT1}, nil))
	variant, err := builder.Variant("Number", builder.Int64(7), variantType)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, variant, variantType)
	value, err := builder.VariantGetUnsafe("Number", variant)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, value, IntT1)
	withDefault, err := builder.VariantGetOrElse("Number", variant, builder.Int64(0))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, withDefault, IntT1)
	if _, err := builder.VariantGetOrElse("Number", variant, builder.Bool(false)); err == nil {
		t.Fatal("incompatible variant fallback accepted")
	}
	setOfVariants := builder.Name("variants", mustSetType(t, variantType))
	filtered, err := builder.VariantFilter("Flag", setOfVariants)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, filtered, mustSetType(t, BoolT1))
}

func TestTypedBuilderDeclarationsLetAndLambda(t *testing.T) {
	builder := NewTypedBuilder()
	parameter, err := NewTypedParameter("x", IntT1)
	if err != nil {
		t.Fatal(err)
	}
	body, err := builder.Apply(Op("PLUS"), builder.Name("x", IntT1), builder.Int64(1))
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := builder.Declaration("Inc", body, parameter)
	if err != nil {
		t.Fatal(err)
	}
	wantDeclarationType := mustOperatorType(t, []Type{IntT1}, IntT1)
	if got, ok := declaration.Tag().Type(); !ok || !EqualTypes(got, wantDeclarationType) {
		t.Fatalf("declaration type = %v, %v", got, ok)
	}
	application, err := builder.ApplyDeclaration(declaration, builder.Int64(41))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, application, IntT1)
	if _, err := builder.ApplyDeclaration(declaration, builder.Bool(true)); err == nil {
		t.Fatal("declaration accepted an argument of the wrong type")
	}
	let, err := builder.LetIn(application, declaration)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, let, IntT1)
	lambda, err := builder.Lambda("LambdaInc", body, parameter)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, lambda, wantDeclarationType)
}

func TestTypedParameterRestrictions(t *testing.T) {
	operatorType := mustOperatorType(t, []Type{IntT1}, BoolT1)
	parameter, err := NewTypedParameter("predicate", operatorType)
	if err != nil || parameter.Parameter.Arity != 1 {
		t.Fatalf("operator parameter = %#v, %v", parameter, err)
	}
	if _, err := NewTypedParameter("nullary", mustOperatorType(t, nil, BoolT1)); err == nil {
		t.Fatal("nullary operator parameter accepted")
	}
	nested := mustSetType(t, operatorType)
	if _, err := NewTypedParameter("nested", nested); err == nil {
		t.Fatal("nested operator type accepted in a parameter")
	}
}
