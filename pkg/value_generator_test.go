package apalache

import (
	"context"
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
