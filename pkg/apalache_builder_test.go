package apalache

import (
	"math/big"
	"testing"
)

func testLambda(t *testing.T, builder *TypedBuilder, name string, argumentTypes []Type, result Type) *LetInExpr {
	t.Helper()
	parameters := make([]TypedParameter, len(argumentTypes))
	for index, typ := range argumentTypes {
		var err error
		parameters[index], err = NewTypedParameter(string(rune('a'+index)), typ)
		if err != nil {
			t.Fatal(err)
		}
	}
	lambda, err := builder.Lambda(name, builder.Name("result", result), parameters...)
	if err != nil {
		t.Fatal(err)
	}
	return lambda
}

func TestTypedApalacheBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			name := builder.Name("x", typ)
			prime, err := builder.Prime(name)
			if err != nil {
				t.Fatal(err)
			}
			right := builder.Name("rhs", typ)
			assignment, err := builder.Assign(prime, right)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, assignment, "Apalache!:=", BoolT1, prime, right)
			if _, err = builder.Assign(builder.Int64(1), builder.Int64(1)); err == nil {
				t.Fatal("Assign accepted an unprimed left-hand side")
			}
			if _, err = builder.Assign(prime, builder.Name("bad", differentConcreteType(typ))); err == nil {
				t.Fatal("Assign accepted incompatible sides")
			}

			generated, err := builder.Gen(builder.Int64(3), typ)
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, generated, typ)
			if generated.Operator().Name() != "Apalache!Gen" {
				t.Fatalf("Gen operator = %s", generated.Operator())
			}

			set := builder.Name("S", mustSetType(t, typ))
			guess, err := builder.Guess(set)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, guess, "Apalache!Guess", typ, set)
			if _, err = builder.Guess(builder.Int64(1)); err == nil {
				t.Fatal("Guess accepted a non-set")
			}

			exists, err := builder.ExistsIn(name, set, builder.Bool(true))
			if err != nil {
				t.Fatal(err)
			}
			skolem, err := builder.Skolem(exists)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, skolem, "Apalache!Skolem", BoolT1, exists)
			if _, err = builder.Skolem(builder.Bool(true)); err == nil {
				t.Fatal("Skolem accepted a non-existential")
			}

			power, err := builder.PowSet(set)
			if err != nil {
				t.Fatal(err)
			}
			expanded, err := builder.Expand(power)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, expanded, "Apalache!Expand", mustSetType(t, mustSetType(t, typ)), power)
			if _, err = builder.Expand(builder.Name("plain", mustSetType(t, mustSetType(t, typ)))); err == nil {
				t.Fatal("Expand accepted a non-constructor")
			}

			cardinality, err := builder.Cardinality(set)
			if err != nil {
				t.Fatal(err)
			}
			comparison, err := builder.Ge(cardinality, builder.Int64(2))
			if err != nil {
				t.Fatal(err)
			}
			constantCardinality, err := builder.ConstCard(comparison)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, constantCardinality, "Apalache!ConstCardinality", BoolT1, comparison)
			if _, err = builder.ConstCard(builder.Bool(true)); err == nil {
				t.Fatal("ConstCard accepted a malformed expression")
			}
		})
	}
}

func TestTypedApalacheBuilderHigherOrderCases(t *testing.T) {
	builder := NewTypedBuilder()
	one := big.NewInt(1)
	unary := testLambda(t, builder, "Unary", []Type{IntT1}, StrT1)
	sequence, err := builder.MkSeq(big.NewInt(3), unary)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, sequence, mustSeqType(t, StrT1))
	if sequence.Operator().Name() != "Apalache!MkSeq" || len(sequence.Arguments()) != 2 || sequence.Arguments()[1] != unary {
		t.Fatal("MkSeq did not preserve its operator argument")
	}
	if _, err = builder.MkSeq(big.NewInt(-1), unary); err == nil {
		t.Fatal("MkSeq accepted a negative count")
	}
	if _, err = builder.MkSeq(one, builder.Name("f", mustOperatorType(t, []Type{IntT1}, StrT1))); err == nil {
		t.Fatal("MkSeq accepted an operator not passed by name")
	}
	constantSequence, err := builder.MkSeqConst(builder.Name("n", IntT1), unary)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, constantSequence, mustSeqType(t, StrT1))

	binary := testLambda(t, builder, "Binary", []Type{IntT1, StrT1}, IntT1)
	set := builder.Name("S", mustSetType(t, StrT1))
	initial := builder.Int64(0)
	foldedSet, err := builder.FoldSet(binary, initial, set)
	if err != nil {
		t.Fatal(err)
	}
	requireActionOperation(t, foldedSet, "Apalache!ApaFoldSet", IntT1, binary, initial, set)
	seq := builder.Name("seq", mustSeqType(t, StrT1))
	foldedSeq, err := builder.FoldSeq(binary, builder.Int64(0), seq)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, foldedSeq, IntT1)
	if _, err = builder.FoldSet(unary, builder.Int64(0), set); err == nil {
		t.Fatal("FoldSet accepted a non-binary lambda")
	}

	repeatLambda := testLambda(t, builder, "Repeat", []Type{IntT1, StrT1}, StrT1)
	repeated, err := builder.Repeat(repeatLambda, big.NewInt(2), builder.Name("s", StrT1))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, repeated, StrT1)
	if _, err = builder.Repeat(repeatLambda, big.NewInt(0), builder.Name("s", StrT1)); err == nil {
		t.Fatal("Repeat accepted a nonpositive count")
	}

	relation := builder.Name("R", mustSetType(t, mustTupleType(t, IntT1, StrT1)))
	function, err := builder.SetAsFun(relation)
	if err != nil {
		t.Fatal(err)
	}
	requireActionOperation(t, function, "Apalache!SetAsFun", mustFunType(t, IntT1, StrT1), relation)
	if _, err = builder.SetAsFun(set); err == nil {
		t.Fatal("SetAsFun accepted a set of non-pairs")
	}
}

func TestApalacheBuilderStrictAndRelaxedModes(t *testing.T) {
	strict := NewTypedBuilder()
	relaxed := NewTypedBuilderWithStrict(false)
	plainBoolean := strict.Bool(true)
	if _, err := strict.Skolem(plainBoolean); err == nil {
		t.Fatal("strict builder accepted malformed Skolem")
	}
	if _, err := relaxed.Skolem(plainBoolean); err != nil {
		t.Fatalf("relaxed builder rejected type-correct Skolem: %v", err)
	}
	setOfSets := relaxed.Name("SS", mustSetType(t, mustSetType(t, IntT1)))
	if _, err := relaxed.Expand(setOfSets); err != nil {
		t.Fatalf("relaxed builder rejected type-correct Expand: %v", err)
	}
}

func TestScopedApalacheBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	assignment, err := builder.Assign(builder.Prime(builder.Name("x", IntT1)), builder.Int64(1)).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, assignment, BoolT1)
	if _, err = builder.Assign(builder.Int64(1), builder.Int64(1)).Build(); err == nil {
		t.Fatal("scoped Assign accepted an unprimed left-hand side")
	}
	generated, err := builder.Gen(builder.Int64(3), StrT1).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, generated, StrT1)
	guess, err := builder.Guess(builder.Name("S", mustSetType(t, IntT1))).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, guess, IntT1)

	typed := NewTypedBuilder()
	unary := testLambda(t, typed, "UnaryScoped", []Type{IntT1}, BoolT1)
	sequence, err := builder.MkSeq(big.NewInt(2), builder.Unchecked(unary)).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, sequence, mustSeqType(t, BoolT1))
	binary := testLambda(t, typed, "BinaryScoped", []Type{IntT1, StrT1}, IntT1)
	folded, err := builder.FoldSet(builder.Unchecked(binary), builder.Int64(0),
		builder.Name("S", mustSetType(t, StrT1))).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, folded, IntT1)
}
