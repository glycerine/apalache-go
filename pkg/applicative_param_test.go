package apalache

import "testing"

func TestApplicativeDomainMatchesScala(t *testing.T) {
	record, _ := NewRecordType(map[string]Type{"field": BoolT1})
	row := mustRowType(t, map[string]Type{"field": BoolT1}, nil)
	sparse, _ := NewSparseTupleType(map[int]Type{3: StrT1})
	for _, test := range []struct {
		name string
		typ  Type
		want Type
	}{
		{"function", mustFunType(t, StrT1, BoolT1), StrT1},
		{"record", record, StrT1},
		{"row", row, StrT1},
		{"record row", NewRecordRowType(row), StrT1},
		{"sequence", mustSeqType(t, BoolT1), IntT1},
		{"sparse tuple", sparse, IntT1},
		{"tuple", mustTupleType(t, IntT1, BoolT1), IntT1},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ApplicativeDomain(test.typ)
			if !ok || !EqualTypes(got, test.want) {
				t.Fatalf("domain = %v, %v; want %v", got, ok, test.want)
			}
		})
	}
	for _, typ := range []Type{IntT1, BoolT1, mustSetType(t, IntT1), mustOperatorType(t, []Type{IntT1}, BoolT1)} {
		if got, ok := ApplicativeDomain(typ); ok {
			t.Errorf("non-applicative %v returned %v", typ, got)
		}
	}
}

func TestApplicativeTypeUsesLiteralHints(t *testing.T) {
	builder := NewTypedBuilder()
	field, _ := builder.Str("field")
	missing, _ := builder.Str("missing")
	record, _ := NewRecordType(map[string]Type{"field": BoolT1})
	row := mustRowType(t, map[string]Type{"field": BoolT1}, nil)
	sparse, _ := NewSparseTupleType(map[int]Type{3: StrT1})
	tuple := mustTupleType(t, IntT1, BoolT1)

	for _, test := range []struct {
		name string
		typ  Type
		hint Expr
		from Type
		to   Type
	}{
		{"function", mustFunType(t, StrT1, BoolT1), builder.Name("x", StrT1), StrT1, BoolT1},
		{"record", record, field, StrT1, BoolT1},
		{"row", row, field, StrT1, BoolT1},
		{"record row", NewRecordRowType(row), field, StrT1, BoolT1},
		{"sequence", mustSeqType(t, BoolT1), builder.Int64(100), IntT1, BoolT1},
		{"sparse tuple", sparse, builder.Int64(3), IntT1, StrT1},
		{"tuple first", tuple, builder.Int64(1), IntT1, IntT1},
		{"tuple second", tuple, builder.Int64(2), IntT1, BoolT1},
	} {
		t.Run(test.name, func(t *testing.T) {
			from, to, ok := ApplicativeType(test.typ, test.hint)
			if !ok || !EqualTypes(from, test.from) || !EqualTypes(to, test.to) {
				t.Fatalf("applicative = (%v, %v, %v), want (%v, %v, true)", from, to, ok, test.from, test.to)
			}
		})
	}
	for _, test := range []struct {
		typ  Type
		hint Expr
	}{
		{record, missing},
		{record, builder.Name("field", StrT1)},
		{sparse, builder.Int64(2)},
		{sparse, builder.Name("index", IntT1)},
		{tuple, builder.Int64(0)},
		{tuple, builder.Int64(3)},
		{tuple, builder.Name("index", IntT1)},
		{IntT1, builder.Int64(1)},
	} {
		if from, to, ok := ApplicativeType(test.typ, test.hint); ok {
			t.Errorf("invalid applicative %v[%v] returned (%v, %v)", test.typ, test.hint, from, to)
		}
	}
}

func TestParameterTypeRestrictionsMatchScala(t *testing.T) {
	variable := mustNumberedVar(t, 7)
	constant := mustNamedConst(t, "MODEL")
	row := mustRowType(t, map[string]Type{"field": IntT1}, &variable)
	sparse, _ := NewSparseTupleType(map[int]Type{1: IntT1})
	record, _ := NewRecordType(map[string]Type{"field": IntT1})
	acceptable := []Type{
		IntT1, StrT1, BoolT1, RealT1, variable, constant,
		mustFunType(t, IntT1, BoolT1), mustSetType(t, IntT1), mustSeqType(t, IntT1),
		mustTupleType(t, IntT1, BoolT1), sparse, record, row, NewRecordRowType(row), NewVariantType(row),
	}
	for _, typ := range acceptable {
		parameter, err := NewTypedParameter("p", typ)
		if err != nil || parameter.Parameter.Arity != 0 || !EqualTypes(parameter.Type, typ) {
			t.Errorf("parameter %v = %#v, %v", typ, parameter, err)
		}
	}

	operator := mustOperatorType(t, []Type{IntT1, BoolT1}, StrT1)
	parameter, err := NewTypedParameter("op", operator)
	if err != nil || parameter.Parameter.Arity != 2 || !EqualTypes(parameter.Type, operator) {
		t.Fatalf("operator parameter = %#v, %v", parameter, err)
	}
	if _, err := NewTypedParameter("nullary", mustOperatorType(t, nil, IntT1)); err == nil {
		t.Fatal("nullary operator parameter was accepted")
	}

	nested := []Type{
		mustFunType(t, operator, IntT1), mustFunType(t, IntT1, operator),
		mustSetType(t, operator), mustSeqType(t, operator), mustTupleType(t, operator),
		mustType(NewSparseTupleType(map[int]Type{1: operator})),
		mustType(NewRecordType(map[string]Type{"field": operator})),
		mustType(NewRowType(map[string]Type{"field": operator}, nil)),
		NewRecordRowType(mustType(NewRowType(map[string]Type{"field": operator}, nil))),
		NewVariantType(mustType(NewRowType(map[string]Type{"Tag": operator}, nil))),
		mustOperatorType(t, []Type{operator}, IntT1), mustOperatorType(t, []Type{IntT1}, operator),
	}
	for _, typ := range nested {
		if _, err := NewTypedParameter("bad", typ); err == nil {
			t.Errorf("nested operator type %v was accepted", typ)
		}
	}

	if err := validateTypedParameter(TypedParameter{Parameter: OperatorParam{Name: "plain", Arity: 0}, Type: operator}); err == nil {
		t.Fatal("operator type with zero syntactic arity was accepted")
	}
	if err := validateTypedParameter(TypedParameter{Parameter: OperatorParam{Name: "higher", Arity: 1}, Type: IntT1}); err == nil {
		t.Fatal("non-operator type with positive syntactic arity was accepted")
	}
}
