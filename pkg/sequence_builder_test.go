package apalache

import "testing"

func differentConcreteType(typ Type) Type {
	if EqualTypes(typ, IntT1) {
		return StrT1
	}
	return IntT1
}

func requireSequenceOperation(t *testing.T, result *OperExpr, operator string, want Type, arguments ...Expr) {
	t.Helper()
	requireActionOperation(t, result, operator, want, arguments...)
}

func TestTypedSequenceBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, elementType := range concreteActionBuilderTypes(t) {
		t.Run(elementType.String(), func(t *testing.T) {
			sequenceType := mustSeqType(t, elementType)
			sequence := builder.Name("seq", sequenceType)
			element := builder.Name("e", elementType)

			appended, err := builder.Append(sequence, element)
			if err != nil {
				t.Fatal(err)
			}
			requireSequenceOperation(t, appended, "Sequences!Append", sequenceType, sequence, element)
			if _, err = builder.Append(builder.Name("bad", IntT1), element); err == nil {
				t.Fatal("Append accepted a non-sequence")
			}
			if _, err = builder.Append(sequence, builder.Name("bad", differentConcreteType(elementType))); err == nil {
				t.Fatal("Append accepted an incompatible element")
			}

			right := builder.Name("right", sequenceType)
			concatenated, err := builder.Concat(sequence, right)
			if err != nil {
				t.Fatal(err)
			}
			requireSequenceOperation(t, concatenated, "Sequences!Concat", sequenceType, sequence, right)
			if _, err = builder.Concat(builder.Name("bad", IntT1), right); err == nil {
				t.Fatal("Concat accepted a non-sequence")
			}
			if _, err = builder.Concat(sequence, builder.Name("bad", mustSeqType(t, differentConcreteType(elementType)))); err == nil {
				t.Fatal("Concat accepted incompatible sequences")
			}

			for _, test := range []struct {
				name     string
				operator string
				want     Type
				build    func(Expr) (*OperExpr, error)
			}{
				{"head", "Sequences!Head", elementType, builder.Head},
				{"tail", "Sequences!Tail", sequenceType, builder.Tail},
				{"len", "Sequences!Len", IntT1, builder.Len},
			} {
				result, buildErr := test.build(sequence)
				if buildErr != nil {
					t.Fatalf("%s: %v", test.name, buildErr)
				}
				requireSequenceOperation(t, result, test.operator, test.want, sequence)
				if _, buildErr = test.build(builder.Name("bad", IntT1)); buildErr == nil {
					t.Errorf("%s accepted a non-sequence", test.name)
				}
			}

			from, to := builder.Name("m", IntT1), builder.Name("n", IntT1)
			subsequence, err := builder.SubSeq(sequence, from, to)
			if err != nil {
				t.Fatal(err)
			}
			requireSequenceOperation(t, subsequence, "Sequences!SubSeq", sequenceType, sequence, from, to)
			if _, err = builder.SubSeq(builder.Name("bad", IntT1), from, to); err == nil {
				t.Fatal("SubSeq accepted a non-sequence")
			}
			if _, err = builder.SubSeq(sequence, builder.Name("bad", StrT1), to); err == nil {
				t.Fatal("SubSeq accepted a non-integer lower bound")
			}
			if _, err = builder.SubSeq(sequence, from, builder.Name("bad", StrT1)); err == nil {
				t.Fatal("SubSeq accepted a non-integer upper bound")
			}
		})
	}
}

func TestScopedSequenceBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	for _, elementType := range concreteActionBuilderTypes(t) {
		t.Run(elementType.String(), func(t *testing.T) {
			sequenceType := mustSeqType(t, elementType)
			sequence := func(name string) ExprInstruction { return builder.Name(name, sequenceType) }

			appended, err := builder.Append(sequence("seq"), builder.Name("e", elementType)).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, appended, sequenceType)
			if appended.(*OperExpr).Operator().Name() != "Sequences!Append" {
				t.Fatalf("append operator = %s", appended.(*OperExpr).Operator())
			}
			if _, err = builder.Append(builder.Name("bad", IntT1), builder.Name("e", elementType)).Build(); err == nil {
				t.Fatal("Append accepted a non-sequence")
			}
			if _, err = builder.Append(sequence("seq"), builder.Name("bad", differentConcreteType(elementType))).Build(); err == nil {
				t.Fatal("Append accepted an incompatible element")
			}

			concatenated, err := builder.Concat(sequence("left"), sequence("right")).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, concatenated, sequenceType)
			if concatenated.(*OperExpr).Operator().Name() != "Sequences!Concat" {
				t.Fatalf("concat operator = %s", concatenated.(*OperExpr).Operator())
			}
			if _, err = builder.Concat(builder.Name("bad", IntT1), sequence("right")).Build(); err == nil {
				t.Fatal("Concat accepted a non-sequence")
			}
			if _, err = builder.Concat(sequence("left"), builder.Name("bad", mustSeqType(t, differentConcreteType(elementType)))).Build(); err == nil {
				t.Fatal("Concat accepted incompatible sequences")
			}

			for _, test := range []struct {
				name     string
				operator string
				want     Type
				build    func(ExprInstruction) ExprInstruction
			}{
				{"head", "Sequences!Head", elementType, builder.Head},
				{"tail", "Sequences!Tail", sequenceType, builder.Tail},
				{"len", "Sequences!Len", IntT1, builder.Len},
			} {
				result, buildErr := test.build(sequence("seq")).Build()
				if buildErr != nil {
					t.Fatalf("%s: %v", test.name, buildErr)
				}
				requireExprType(t, result, test.want)
				if result.(*OperExpr).Operator().Name() != test.operator {
					t.Fatalf("%s operator = %s", test.name, result.(*OperExpr).Operator())
				}
				if _, buildErr = test.build(builder.Name("bad", IntT1)).Build(); buildErr == nil {
					t.Errorf("%s accepted a non-sequence", test.name)
				}
			}

			subsequence, err := builder.SubSeq(sequence("seq"), builder.Name("m", IntT1), builder.Name("n", IntT1)).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, subsequence, sequenceType)
			if subsequence.(*OperExpr).Operator().Name() != "Sequences!SubSeq" {
				t.Fatalf("subseq operator = %s", subsequence.(*OperExpr).Operator())
			}
			if _, err = builder.SubSeq(builder.Name("bad", IntT1), builder.Name("m", IntT1), builder.Name("n", IntT1)).Build(); err == nil {
				t.Fatal("SubSeq accepted a non-sequence")
			}
			if _, err = builder.SubSeq(sequence("seq"), builder.Name("bad", StrT1), builder.Name("n", IntT1)).Build(); err == nil {
				t.Fatal("SubSeq accepted a non-integer lower bound")
			}
			if _, err = builder.SubSeq(sequence("seq"), builder.Name("m", IntT1), builder.Name("bad", StrT1)).Build(); err == nil {
				t.Fatal("SubSeq accepted a non-integer upper bound")
			}
		})
	}
}
