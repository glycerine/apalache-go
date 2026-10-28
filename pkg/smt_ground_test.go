package apalache

import (
	"context"
	"math/big"
	"testing"
)

func declareArenaForEncoder(t *testing.T, encoder *G3GroundEncoder, arena *PureArena) {
	t.Helper()
	for _, name := range []string{ArenaFalseName, ArenaTrueName, ArenaBooleanSet, ArenaNatSet, ArenaIntSet} {
		cell, err := arena.FindCellByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := encoder.DeclareCell(cell); err != nil {
			t.Fatal(err)
		}
	}
}

func TestG3GroundEncoderBooleanArithmeticAndMembership(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, err := NewG3GroundEncoder(solver)
	if err != nil {
		t.Fatal(err)
	}
	arena := InitialPureArena()
	declareArenaForEncoder(t, encoder, arena)
	if err := encoder.DeclareMembership(arena.BooleanSetCell(), arena.FalseCell()); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), arena.FalseCell().NameExpr())); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(arena.TrueCell().NameExpr()); err != nil {
		t.Fatal(err)
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), arena.FalseCell().NameExpr(), arena.BooleanSetCell().NameExpr())
	if err := encoder.Assert(membership); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("base ground encoding = %v, %v", status, err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), membership)); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("contradictory membership = %v, %v", status, err)
	}
}

func TestG3GroundEncoderBigIntegersAndUninterpretedDistinct(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, _ := NewG3GroundEncoder(solver)
	arena := EmptyPureArena()
	arena, integer := arena.AppendCellType(BasicCellType{Type: IntT1})
	typeA, _ := NewConstType("A")
	arena, values := arena.AppendCellTypes(BasicCellType{Type: typeA}, BasicCellType{Type: typeA})
	for _, cell := range append([]ArenaCell{integer}, values...) {
		if err := encoder.DeclareCell(cell); err != nil {
			t.Fatal(err)
		}
	}
	huge := new(big.Int).Exp(big.NewInt(2), big.NewInt(100), nil)
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1), integer.NameExpr(), NewValueExpr(NewIntValue(huge), Typed(IntT1)))
	if err := encoder.Assert(equality); err != nil {
		t.Fatal(err)
	}
	distinct := MustOperExpr(Op("Apalache!Distinct"), Typed(BoolT1), values[0].NameExpr(), values[1].NameExpr())
	if err := encoder.Assert(distinct); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("EQ"), Typed(BoolT1), values[0].NameExpr(), values[1].NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("uninterpreted distinct contradiction = %v, %v", status, err)
	}
	if len(encoder.DeclaredCellNames()) != arena.CellCount() {
		t.Fatalf("declared cell names = %v", encoder.DeclaredCellNames())
	}
}

func TestG3GroundEncoderRejectsMissingAndUnsupportedTerms(t *testing.T) {
	encoder, err := NewG3GroundEncoder(newTestG3Solver(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := encoder.Encode(NewNameExpr("$C$99", Typed(IntT1))); err == nil {
		t.Fatal("undeclared arena cell was encoded")
	}
	if _, err := encoder.Encode(NewValueExpr(StringValue("x"), Typed(StrT1))); err == nil {
		t.Fatal("raw string literal was encoded")
	}
	cell := NewArenaCell(0, BasicCellType{Type: IntT1})
	set := NewArenaCell(1, BasicCellType{Type: mustSet(IntT1)})
	if err := encoder.DeclareCell(cell); err != nil {
		t.Fatal(err)
	}
	if err := encoder.DeclareCell(set); err != nil {
		t.Fatal(err)
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), cell.NameExpr(), set.NameExpr())
	if _, err := encoder.Encode(membership); err == nil {
		t.Fatal("undeclared membership predicate was encoded")
	}
	if err := encoder.DeclareCell(cell); err == nil {
		t.Fatal("duplicate cell declaration was accepted")
	}
}

func TestG3GroundEncoderUsesOOPSLAEdgesForFunctionMembership(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, err := NewG3GroundEncoder(solver)
	if err != nil {
		t.Fatal(err)
	}
	element := NewArenaCell(0, BasicCellType{Type: IntT1})
	function := NewArenaCell(1, BasicCellType{Type: mustFunType(t, IntT1, BoolT1)})
	if err = encoder.DeclareCell(element); err != nil {
		t.Fatal(err)
	}
	if err = encoder.DeclareCell(function); err != nil {
		t.Fatal(err)
	}
	if err = encoder.DeclareMembership(function, element); err != nil {
		t.Fatal(err)
	}
	selected := MustOperExpr(Op("Apalache!SelectInFun"), Typed(BoolT1), element.NameExpr(), function.NameExpr())
	notStored := MustOperExpr(Op("Apalache!storeNotInFun"), Typed(BoolT1), element.NameExpr(), function.NameExpr())
	if err = encoder.Assert(selected); err != nil {
		t.Fatal(err)
	}
	if err = encoder.Assert(notStored); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("function edge contradiction = %v, %v", status, checkErr)
	}
}

func TestG3GroundEncoderArraySetDeclarationsAndSelection(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, err := NewG3GroundEncoderWithEncoding(solver, SMTEncodingArrays)
	if err != nil {
		t.Fatal(err)
	}
	element := NewArenaCell(0, BasicCellType{Type: IntT1})
	emptySet := NewArenaCell(1, BasicCellType{Type: mustSet(IntT1)})
	fullSet := NewArenaCell(2, InfiniteSetCellType{Element: BasicCellType{Type: IntT1}})
	for _, cell := range []ArenaCell{element, emptySet, fullSet} {
		if err = encoder.DeclareCell(cell); err != nil {
			t.Fatal(err)
		}
	}
	selectFrom := func(set ArenaCell) Expr {
		return MustOperExpr(Op("Apalache!SelectInSet"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
	}
	contradiction := MustOperExpr(Op("OR"), Typed(BoolT1),
		selectFrom(emptySet), MustOperExpr(Op("NOT"), Typed(BoolT1), selectFrom(fullSet)))
	if err = encoder.Assert(contradiction); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("constant set arrays = %v, %v", status, checkErr)
	}
}
