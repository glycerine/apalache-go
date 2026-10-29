package apalache

import (
	"context"
	"fmt"
	"testing"
)

func TestStratifiedGenBuildsBoundedStructures(t *testing.T) {
	row, _ := NewRowType(map[string]Type{"A": IntT1, "B": BoolT1}, nil)
	variantType := NewVariantType(row)
	recordRow, _ := NewRowType(map[string]Type{"x": IntT1, "ok": BoolT1}, nil)
	recordRowType := NewRecordRowType(recordRow)
	recordType, _ := NewRecordType(map[string]Type{"x": IntT1, "ok": BoolT1})
	tupleType, _ := NewTupleType(IntT1, BoolT1)
	sequenceType, _ := NewSeqType(IntT1)
	functionType, _ := NewFunType(IntT1, BoolT1)
	constantType, _ := NewConstType("PID")
	types := []Type{
		IntT1, BoolT1, StrT1, constantType, mustSet(IntT1), sequenceType,
		tupleType, recordType, recordRowType, variantType, functionType,
	}
	for _, typ := range types {
		t.Run(typ.String(), func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			expression := MustOperExpr(Op("Apalache!Gen"), Typed(typ),
				NewValueExpr(Int64Value(2), Typed(IntT1)))
			scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
			if err != nil || !EqualTypes(result.Type.TLAType(), typ) {
				t.Fatalf("Gen(%s) = %v, %v", typ, result, err)
			}
			if _, err := EncodeRewriterScope(newTestG3Solver(t), scope, rewriter.Constraints()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStratifiedGenScalaOracleCases(t *testing.T) {
	integer := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	stringValue := func(value string) Expr { return NewValueExpr(StringValue(value), Typed(StrT1)) }
	equal := func(left, right Expr) Expr { return MustOperExpr(Op("EQ"), Typed(BoolT1), left, right) }
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = integer(value)
		}
		return MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), elements...)
	}
	check := func(t *testing.T, typ Type, bound int64, claim func(Expr) Expr, want SMTStatus) {
		t.Helper()
		rewriter := NewStratifiedRewriter()
		scope, generated, err := rewriter.Rewrite(
			MustOperExpr(Op("Apalache!Gen"), Typed(typ), integer(bound)), InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		scope, result, err := rewriter.Rewrite(claim(generated.NameExpr()), scope)
		if err != nil {
			t.Fatal(err)
		}
		solver := newTestG3Solver(t)
		encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
		if err != nil {
			t.Fatal(err)
		}
		if err = encoder.Assert(result.NameExpr()); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != want {
			t.Fatalf("generated %s claim = %v, want %v, error %v", typ, status, want, checkErr)
		}
	}

	t.Run("integer admits large value", func(t *testing.T) {
		check(t, IntT1, 1, func(generated Expr) Expr {
			return MustOperExpr(Op("GE"), Typed(BoolT1), generated, integer(10))
		}, SMTSatisfiable)
	})
	for _, test := range []struct {
		name    string
		typ     Type
		negated bool
	}{
		{name: "string admits literal", typ: StrT1},
		{name: "string admits other literal", typ: StrT1, negated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			check(t, test.typ, 1, func(generated Expr) Expr {
				comparison := equal(generated.WithTag(Typed(StrT1)), stringValue("foo"))
				if test.negated {
					return MustOperExpr(Op("NOT"), Typed(BoolT1), comparison)
				}
				return comparison
			}, SMTSatisfiable)
		})
	}
	constantType, err := NewConstType("PID")
	if err != nil {
		t.Fatal(err)
	}
	for _, negated := range []bool{false, true} {
		t.Run(fmt.Sprintf("constant literal negated=%v", negated), func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope, generated, rewriteErr := rewriter.Rewrite(
				MustOperExpr(Op("Apalache!Gen"), Typed(constantType), integer(1)), InitialRewriterScope())
			if rewriteErr != nil {
				t.Fatal(rewriteErr)
			}
			scope, literal, rewriteErr := rewriter.Rewrite(stringValue("foo"), scope)
			if rewriteErr != nil {
				t.Fatal(rewriteErr)
			}
			solver := newTestG3Solver(t)
			encoder, encodeErr := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			claim := equal(generated.NameExpr().WithTag(Typed(StrT1)), literal.NameExpr())
			if negated {
				claim = MustOperExpr(Op("NOT"), Typed(BoolT1), claim)
			}
			if assertErr := encoder.Assert(claim); assertErr != nil {
				t.Fatal(assertErr)
			}
			if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
				t.Fatalf("generated PID literal claim = %v, %v", status, checkErr)
			}
		})
	}
	for _, value := range []bool{false, true} {
		t.Run(fmt.Sprintf("boolean admits %v", value), func(t *testing.T) {
			check(t, BoolT1, 1, func(generated Expr) Expr {
				if value {
					return generated
				}
				return MustOperExpr(Op("NOT"), Typed(BoolT1), generated)
			}, SMTSatisfiable)
		})
	}
	for _, values := range [][]int64{{1, 2, 3}, {}} {
		t.Run(fmt.Sprintf("set admits %v", values), func(t *testing.T) {
			check(t, mustSet(IntT1), 3, func(generated Expr) Expr {
				return equal(generated, set(values...))
			}, SMTSatisfiable)
		})
	}

	recordType, _ := NewRecordType(map[string]Type{"i": IntT1, "b": BoolT1})
	row, _ := NewRowType(map[string]Type{"i": IntT1, "b": BoolT1}, nil)
	rowRecordType := NewRecordRowType(row)
	for _, typ := range []Type{recordType, rowRecordType} {
		t.Run("record "+typ.String(), func(t *testing.T) {
			check(t, typ, 3, func(generated Expr) Expr {
				field := MustOperExpr(Op("FUN_APP"), Typed(IntT1), generated, stringValue("i"))
				return equal(field, integer(10))
			}, SMTSatisfiable)
		})
	}

	variantRow, _ := NewRowType(map[string]Type{"Foo": IntT1, "Bar": BoolT1}, nil)
	variantType := NewVariantType(variantRow)
	for _, test := range []struct {
		name    string
		witness Expr
	}{
		{name: "Foo", witness: testVariantExpression(variantType, "Foo", integer(3))},
		{name: "Bar", witness: testVariantExpression(variantType, "Bar", boolExpr(true))},
	} {
		t.Run("variant admits "+test.name, func(t *testing.T) {
			check(t, variantType, 3, func(generated Expr) Expr { return equal(generated, test.witness) }, SMTSatisfiable)
		})
	}

	tupleType, _ := NewTupleType(IntT1, BoolT1)
	t.Run("tuple field admits value", func(t *testing.T) {
		check(t, tupleType, 3, func(generated Expr) Expr {
			return equal(MustOperExpr(Op("FUN_APP"), Typed(IntT1), generated, integer(1)), integer(10))
		}, SMTSatisfiable)
	})

	functionType, _ := NewFunType(IntT1, BoolT1)
	t.Run("function domain witness", func(t *testing.T) {
		check(t, functionType, 4, func(generated Expr) Expr {
			return equal(MustOperExpr(Op("DOMAIN"), Typed(mustSet(IntT1)), generated), set(1, 3))
		}, SMTSatisfiable)
	})
	t.Run("function is single valued", func(t *testing.T) {
		check(t, functionType, 4, func(generated Expr) Expr {
			domain := equal(MustOperExpr(Op("DOMAIN"), Typed(mustSet(IntT1)), generated), set(1, 3))
			one := MustOperExpr(Op("FUN_APP"), Typed(BoolT1), generated, integer(1))
			oneAgain := MustOperExpr(Op("FUN_APP"), Typed(BoolT1), generated,
				MustOperExpr(Op("MINUS"), Typed(IntT1), integer(2), integer(1)))
			return MustOperExpr(Op("AND"), Typed(BoolT1), domain,
				MustOperExpr(Op("NOT"), Typed(BoolT1), equal(one, oneAgain)))
		}, SMTUnsatisfiable)
	})

	sequenceType, _ := NewSeqType(BoolT1)
	t.Run("sequence domain witness", func(t *testing.T) {
		check(t, sequenceType, 4, func(generated Expr) Expr {
			return equal(MustOperExpr(Op("DOMAIN"), Typed(mustSet(IntT1)), generated), set(1, 2, 3))
		}, SMTSatisfiable)
	})
}

func TestStratifiedGenEnforcesSetAndSequenceBounds(t *testing.T) {
	intSetType := mustSet(IntT1)
	sequenceType, _ := NewSeqType(IntT1)
	tests := []struct {
		name string
		typ  Type
		bad  func(ArenaCell) Expr
	}{
		{
			"set cardinality",
			intSetType,
			func(cell ArenaCell) Expr {
				cardinality := MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), cell.NameExpr())
				return MustOperExpr(Op("GT"), Typed(BoolT1), cardinality,
					NewValueExpr(Int64Value(2), Typed(IntT1)))
			},
		},
		{
			"sequence length",
			sequenceType,
			func(cell ArenaCell) Expr {
				length := MustOperExpr(Op("Sequences!Len"), Typed(IntT1), cell.NameExpr())
				return MustOperExpr(Op("GT"), Typed(BoolT1), length,
					NewValueExpr(Int64Value(2), Typed(IntT1)))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope, generated, err := rewriter.Rewrite(MustOperExpr(Op("Apalache!Gen"), Typed(test.typ),
				NewValueExpr(Int64Value(2), Typed(IntT1))), InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			scope.Binding = scope.Binding.With("generated", generated)
			bad := test.bad(generated)
			scope, result, err := rewriter.Rewrite(bad, scope)
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(result.NameExpr()); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("Gen bound violation = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedGenVariantTagIsAnOption(t *testing.T) {
	row, _ := NewRowType(map[string]Type{"A": IntT1, "B": BoolT1}, nil)
	variantType := NewVariantType(row)
	generated := MustOperExpr(Op("Apalache!Gen"), Typed(variantType),
		NewValueExpr(Int64Value(1), Typed(IntT1)))
	rewriter := NewStratifiedRewriter()
	scope, variant, err := rewriter.Rewrite(generated, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	scope.Binding = scope.Binding.With("v", variant)
	tag := MustOperExpr(Op("Variants!VariantTag"), Typed(StrT1), NewNameExpr("v", Typed(variantType)))
	valid := MustOperExpr(Op("OR"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), tag, NewValueExpr(StringValue("A"), Typed(StrT1))),
		MustOperExpr(Op("EQ"), Typed(BoolT1), tag, NewValueExpr(StringValue("B"), Typed(StrT1))))
	scope, result, err := rewriter.Rewrite(valid, scope)
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedGenFunctionHasUniqueActiveKeys(t *testing.T) {
	functionType, _ := NewFunType(IntT1, BoolT1)
	rewriter := NewStratifiedRewriter()
	scope, function, err := rewriter.Rewrite(MustOperExpr(Op("Apalache!Gen"), Typed(functionType),
		NewValueExpr(Int64Value(2), Typed(IntT1))), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	relation, err := scope.Arena.Codomain(function)
	if err != nil {
		t.Fatal(err)
	}
	pointers := scope.Arena.GetHas(relation)
	if len(pointers) != 2 {
		t.Fatalf("generated relation has %d entries, want 2", len(pointers))
	}
	firstKey, _, err := unpackFunctionPair(scope.Arena, pointers[0].Element())
	if err != nil {
		t.Fatal(err)
	}
	secondKey, _, err := unpackFunctionPair(scope.Arena, pointers[1].Element())
	if err != nil {
		t.Fatal(err)
	}
	equal, err := rewriter.equality.Equal(scope.Arena, firstKey, secondKey)
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	constraints := append(rewriter.Constraints(), pointers[0].ToSMT(), pointers[1].ToSMT(), equal)
	if _, err := EncodeRewriterScope(solver, scope, constraints); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("duplicate active generated function keys = %v, %v", status, err)
	}
}

func TestStratifiedGenRejectsInvalidBounds(t *testing.T) {
	invalid := []Expr{
		MustOperExpr(Op("Apalache!Gen"), Typed(IntT1), NewValueExpr(Int64Value(-1), Typed(IntT1))),
		MustOperExpr(Op("Apalache!Gen"), Typed(IntT1), NewValueExpr(Int64Value(0), Typed(IntT1))),
		MustOperExpr(Op("Apalache!Gen"), Untyped(), NewValueExpr(Int64Value(1), Typed(IntT1))),
		MustOperExpr(Op("Apalache!Gen"), Typed(IntT1), NewValueExpr(Int64Value(1), Typed(BoolT1))),
		MustOperExpr(Op("Apalache!Gen"), Typed(IntT1), NewNameExpr("n", Typed(IntT1))),
	}
	for _, expression := range invalid {
		base := InitialRewriterScope()
		returned, _, err := NewStratifiedRewriter().Rewrite(expression, base)
		if err == nil || returned.Arena != base.Arena {
			t.Fatalf("invalid Gen returned scope=%v, err=%v", returned, err)
		}
	}
}
