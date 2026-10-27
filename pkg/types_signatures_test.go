package apalache

import (
	"errors"
	"testing"
)

func TestRepresentativeOperatorSignatures(t *testing.T) {
	setInt := mustSetType(t, IntT1)
	setBool := mustSetType(t, BoolT1)
	seqInt := mustSeqType(t, IntT1)
	intToBool := mustFunType(t, IntT1, BoolT1)
	intIntToInt := mustOperatorType(t, []Type{IntT1, IntT1}, IntT1)
	variant := NewVariantType(mustRowType(t, map[string]Type{"A": IntT1}, nil))
	tuplePair := mustTupleType(t, IntT1, BoolT1)

	tests := []struct {
		name string
		args []Type
		want Type
	}{
		{"EQ", []Type{IntT1, IntT1}, BoolT1},
		{"OPER_APP", []Type{mustOperatorType(t, []Type{IntT1}, BoolT1), IntT1}, BoolT1},
		{"CHOOSE3", []Type{IntT1, setInt, BoolT1}, IntT1},
		{"AND", nil, BoolT1},
		{"PLUS", []Type{IntT1, IntT1}, IntT1},
		{"INT_RANGE", []Type{IntT1, IntT1}, setInt},
		{"IF_THEN_ELSE", []Type{BoolT1, IntT1, IntT1}, IntT1},
		{"CASE", []Type{BoolT1, IntT1, BoolT1, IntT1}, IntT1},
		{"CASE_OTHER", []Type{IntT1, BoolT1, IntT1}, IntT1},
		{"SET_ENUM", []Type{IntT1, IntT1}, setInt},
		{"SET_UNION2", []Type{setInt, setInt}, setInt},
		{"SET_MAP", []Type{BoolT1, IntT1, setInt}, setBool},
		{"Sequences!Seq", []Type{setInt}, mustSetType(t, seqInt)},
		{"SET_TIMES", []Type{setInt, setBool}, mustSetType(t, tuplePair)},
		{"FUN_SET", []Type{setInt, setBool}, mustSetType(t, intToBool)},
		{"FUN_CTOR", []Type{BoolT1, IntT1, setInt}, intToBool},
		{"Sequences!Append", []Type{seqInt, IntT1}, seqInt},
		{"FiniteSets!Cardinality", []Type{setInt}, IntT1},
		{"PRIME", []Type{IntT1}, IntT1},
		{"GLOBALLY", []Type{BoolT1}, BoolT1},
		{"Variants!VariantTag", []Type{variant}, StrT1},
		{"Apalache!Guess", []Type{setInt}, IntT1},
		{"Apalache!ApaFoldSet", []Type{intIntToInt, IntT1, setInt}, IntT1},
		{"Apalache!Distinct", []Type{IntT1, IntT1}, BoolT1},
		{"Apalache!SelectInFun", []Type{IntT1, intToBool}, BoolT1},
		{"Apalache!StoreInSet", []Type{BoolT1, intToBool, IntT1}, BoolT1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := InferOperatorType(Op(test.name), test.args)
			if err != nil || !EqualTypes(got, test.want) {
				t.Fatalf("InferOperatorType() = %v, %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestSignaturesUseFlexibleRecordEquality(t *testing.T) {
	left, _ := NewRecordType(map[string]Type{"left": IntT1})
	right, _ := NewRecordType(map[string]Type{"right": BoolT1})
	union, _ := NewRecordType(map[string]Type{"left": IntT1, "right": BoolT1})
	got, err := InferOperatorType(Op("IF_THEN_ELSE"), []Type{BoolT1, left, right})
	if err != nil || !EqualTypes(got, union) {
		t.Fatalf("record branch result = %v, %v; want %v", got, err, union)
	}
	got, err = InferOperatorType(Op("SET_ENUM"), []Type{left, right})
	if err != nil || !EqualTypes(got, mustSetType(t, union)) {
		t.Fatalf("record enumeration result = %v, %v", got, err)
	}
}

func TestOperatorSignatureFailuresAndUnknowns(t *testing.T) {
	setInt := mustSetType(t, IntT1)
	for _, test := range []struct {
		name string
		args []Type
	}{
		{"PLUS", []Type{IntT1, BoolT1}},
		{"SET_IN", []Type{BoolT1, setInt}},
		{"SET_ENUM", nil},
		{"CASE", []Type{IntT1, IntT1}},
		{"Sequences!Head", []Type{IntT1}},
		{"Apalache!Repeat", []Type{IntT1, IntT1, IntT1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if typ, err := InferOperatorType(Op(test.name), test.args); err == nil {
				t.Fatalf("invalid signature returned %v", typ)
			} else {
				var typeError *OperatorTypeError
				if !errors.As(err, &typeError) {
					t.Fatalf("error type = %T, want *OperatorTypeError", err)
				}
			}
		})
	}

	for _, operator := range []Operator{Op("FUN_APP"), Op("TUPLE"), Op("RECORD"), Op("Apalache!Gen"), NewSMTMapOperator(Op("PLUS"))} {
		if _, err := InferOperatorType(operator, make([]Type, signatureTestArity(operator))); !errors.Is(err, ErrUnknownOperatorSignature) {
			t.Errorf("%s unknown signature error = %v", operator.Name(), err)
		}
	}
}

func signatureTestArity(operator Operator) int {
	for arity := 0; arity < 8; arity++ {
		if operator.IsCorrectArity(arity) {
			return arity
		}
	}
	return 0
}

func TestSMTMapSignatures(t *testing.T) {
	setInt := mustSetType(t, IntT1)
	for _, mapped := range []string{"AND", "OR"} {
		operator := NewSMTMapOperator(Op(mapped))
		got, err := InferOperatorType(operator, []Type{setInt, setInt})
		if err != nil || !EqualTypes(got, setInt) {
			t.Errorf("%s signature = %v, %v", operator.Name(), got, err)
		}
	}
}
