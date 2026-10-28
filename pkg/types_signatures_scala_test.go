package apalache

import "testing"

func TestScalaPureSignatureMaps(t *testing.T) {
	setInt := mustSetType(t, IntT1)
	setBool := mustSetType(t, BoolT1)
	setSetInt := mustSetType(t, setInt)
	seqInt := mustSeqType(t, IntT1)
	seqBool := mustSeqType(t, BoolT1)
	funIntBool := mustFunType(t, IntT1, BoolT1)
	opIntIntInt := mustOperatorType(t, []Type{IntT1, IntT1}, IntT1)
	opIntBool := mustOperatorType(t, []Type{IntT1}, BoolT1)
	variant := NewVariantType(mustRowType(t, map[string]Type{"A": IntT1}, nil))
	pair := mustTupleType(t, IntT1, BoolT1)

	tests := []struct {
		name string
		args []Type
		want Type
	}{
		// ActionOperSignatures
		{"PRIME", []Type{IntT1}, IntT1},
		{"STUTTER", []Type{BoolT1, IntT1}, BoolT1},
		{"NO_STUTTER", []Type{BoolT1, IntT1}, BoolT1},
		{"ENABLED", []Type{BoolT1}, BoolT1},
		{"UNCHANGED", []Type{IntT1}, BoolT1},
		{"COMPOSE", []Type{BoolT1, BoolT1}, BoolT1},

		// ApalacheInternalOperSignatures (manual-result unsupported node omitted).
		{"Apalache!Distinct", nil, BoolT1},
		{"ApalacheInternal!__ApalacheSeqCapacity", []Type{seqInt}, IntT1},
		{"Apalache!SelectInSet", []Type{IntT1, setInt}, BoolT1},
		{"Apalache!SelectInFun", []Type{IntT1, funIntBool}, BoolT1},
		{"Apalache!StoreInSet", []Type{IntT1, setInt}, BoolT1},
		{"Apalache!StoreInSet", []Type{BoolT1, funIntBool, IntT1}, BoolT1},
		{"Apalache!storeNotInSet", []Type{IntT1, setInt}, BoolT1},
		{"Apalache!storeNotInFun", []Type{IntT1, funIntBool}, BoolT1},
		{"Apalache!UnconstrainArray", []Type{setInt}, BoolT1},

		// ApalacheOperSignatures (manual-result Gen omitted).
		{"Apalache!:=", []Type{IntT1, IntT1}, BoolT1},
		{"Apalache!Skolem", []Type{BoolT1}, BoolT1},
		{"Apalache!Guess", []Type{setInt}, IntT1},
		{"Apalache!Expand", []Type{setSetInt}, setSetInt},
		{"Apalache!ConstCardinality", []Type{BoolT1}, BoolT1},
		{"Apalache!MkSeq", []Type{IntT1, opIntBool}, seqBool},
		{"Apalache!Repeat", []Type{mustOperatorType(t, []Type{IntT1, BoolT1}, BoolT1), IntT1, BoolT1}, BoolT1},
		{"Apalache!ApaFoldSet", []Type{opIntIntInt, IntT1, setInt}, IntT1},
		{"Apalache!ApaFoldSeqLeft", []Type{opIntIntInt, IntT1, seqInt}, IntT1},
		{"Apalache!SetAsFun", []Type{mustSetType(t, pair)}, funIntBool},

		// ArithOperSignatures
		{"PLUS", []Type{IntT1, IntT1}, IntT1}, {"MINUS", []Type{IntT1, IntT1}, IntT1},
		{"MULT", []Type{IntT1, IntT1}, IntT1}, {"DIV", []Type{IntT1, IntT1}, IntT1},
		{"MOD", []Type{IntT1, IntT1}, IntT1}, {"POW", []Type{IntT1, IntT1}, IntT1},
		{"LT", []Type{IntT1, IntT1}, BoolT1}, {"GT", []Type{IntT1, IntT1}, BoolT1},
		{"LE", []Type{IntT1, IntT1}, BoolT1}, {"GE", []Type{IntT1, IntT1}, BoolT1},
		{"UNARY_MINUS", []Type{IntT1}, IntT1}, {"INT_RANGE", []Type{IntT1, IntT1}, setInt},

		// BaseOperSignatures
		{"EQ", []Type{IntT1, IntT1}, BoolT1}, {"NE", []Type{IntT1, IntT1}, BoolT1},
		{"OPER_APP", []Type{opIntBool, IntT1}, BoolT1},
		{"CHOOSE3", []Type{IntT1, setInt, BoolT1}, IntT1},
		{"CHOOSE2", []Type{IntT1, BoolT1}, IntT1},
		{"LABEL", []Type{IntT1, StrT1}, IntT1},

		// BoolOperSignatures
		{"AND", nil, BoolT1}, {"OR", []Type{BoolT1, BoolT1}, BoolT1},
		{"IMPLIES", []Type{BoolT1, BoolT1}, BoolT1}, {"EQUIV", []Type{BoolT1, BoolT1}, BoolT1},
		{"FORALL3", []Type{IntT1, setInt, BoolT1}, BoolT1}, {"EXISTS3", []Type{IntT1, setInt, BoolT1}, BoolT1},
		{"FORALL2", []Type{IntT1, BoolT1}, BoolT1}, {"EXISTS2", []Type{IntT1, BoolT1}, BoolT1},
		{"NOT", []Type{BoolT1}, BoolT1},

		// ControlOperSignatures
		{"IF_THEN_ELSE", []Type{BoolT1, IntT1, IntT1}, IntT1},
		{"CASE", []Type{BoolT1, IntT1, BoolT1, IntT1}, IntT1},
		{"CASE_OTHER", []Type{IntT1, BoolT1, IntT1}, IntT1},

		// FiniteSetOperSignatures
		{"FiniteSets!IsFiniteSet", []Type{setInt}, BoolT1},
		{"FiniteSets!Cardinality", []Type{setInt}, IntT1},

		// FunOperSignatures
		{"FUN_CTOR", []Type{BoolT1, IntT1, setInt}, funIntBool},

		// SeqOperSignatures
		{"Sequences!Head", []Type{seqInt}, IntT1}, {"Sequences!Tail", []Type{seqInt}, seqInt},
		{"Sequences!Append", []Type{seqInt, IntT1}, seqInt}, {"Sequences!Concat", []Type{seqInt, seqInt}, seqInt},
		{"Sequences!Len", []Type{seqInt}, IntT1}, {"Sequences!SubSeq", []Type{seqInt, IntT1, IntT1}, seqInt},

		// SetOperSignatures
		{"SET_UNION2", []Type{setInt, setInt}, setInt}, {"SET_INTERSECT", []Type{setInt, setInt}, setInt},
		{"SET_MINUS", []Type{setInt, setInt}, setInt}, {"SET_IN", []Type{IntT1, setInt}, BoolT1},
		{"SET_NOT_IN", []Type{IntT1, setInt}, BoolT1}, {"SET_MAP", []Type{BoolT1, IntT1, setInt}, setBool},
		{"SET_FILTER", []Type{IntT1, setInt, BoolT1}, setInt},
		{"Sequences!Seq", []Type{setInt}, mustSetType(t, seqInt)},
		{"SET_POWERSET", []Type{setInt}, setSetInt},
		{"SET_TIMES", []Type{setInt, setBool}, mustSetType(t, pair)},
		{"FUN_SET", []Type{setInt, setBool}, mustSetType(t, funIntBool)},
		{"SET_UNARY_UNION", []Type{setSetInt}, setInt}, {"SET_ENUM", []Type{IntT1, IntT1}, setInt},
		{"SET_SUBSET_EQ", []Type{setInt, setInt}, BoolT1},

		// TemporalOperSignatures
		{"GLOBALLY", []Type{BoolT1}, BoolT1}, {"EVENTUALLY", []Type{BoolT1}, BoolT1},
		{"LEADS_TO", []Type{BoolT1, BoolT1}, BoolT1}, {"GUARANTEES", []Type{BoolT1, BoolT1}, BoolT1},
		{"WEAK_FAIRNESS", []Type{IntT1, BoolT1}, BoolT1}, {"STRONG_FAIRNESS", []Type{IntT1, BoolT1}, BoolT1},
		{"TEMPORAL_EXISTS", []Type{IntT1, BoolT1}, BoolT1}, {"TEMPORAL_FORALL", []Type{IntT1, BoolT1}, BoolT1},

		// VariantOperSignatures
		{"Variants!VariantTag", []Type{variant}, StrT1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := InferOperatorType(Op(test.name), test.args)
			if err != nil || !EqualTypes(got, test.want) {
				t.Fatalf("signature(%v) = %v, %v; want %v", test.args, got, err, test.want)
			}
		})
	}

	for _, mapped := range []Operator{Op("AND"), Op("OR")} {
		got, err := InferOperatorType(NewSMTMapOperator(mapped), []Type{setInt, setInt})
		if err != nil || !EqualTypes(got, setInt) {
			t.Fatalf("SMT map %s = %v, %v", mapped, got, err)
		}
	}
}

func TestScalaSignatureFidelityEdges(t *testing.T) {
	left, _ := NewRecordType(map[string]Type{"left": IntT1})
	right, _ := NewRecordType(map[string]Type{"right": BoolT1})
	wide, _ := NewRecordType(map[string]Type{"left": IntT1, "right": BoolT1})

	// Repeat returns its declared accumulator parameter, not the widened common type.
	repeater := mustOperatorType(t, []Type{IntT1, left}, right)
	got, err := InferOperatorType(Op("Apalache!Repeat"), []Type{repeater, IntT1, wide})
	if err != nil || !EqualTypes(got, left) {
		t.Fatalf("Repeat result = %v, %v; want original accumulator %v", got, err, left)
	}

	// CASE OTHER unifies explicit bodies first, then requires exact OTHER equality.
	if _, err := InferOperatorType(Op("CASE_OTHER"), []Type{left, BoolT1, right}); err == nil {
		t.Fatal("CASE OTHER accepted OTHER unequal to the explicit-body common type")
	}
	got, err = InferOperatorType(Op("CASE_OTHER"), []Type{wide, BoolT1, left, BoolT1, right})
	if err != nil || !EqualTypes(got, wide) {
		t.Fatalf("CASE OTHER common body = %v, %v; want %v", got, err, wide)
	}
}
