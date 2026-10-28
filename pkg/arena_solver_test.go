package apalache

import (
	"context"
	"testing"
)

func TestArenaSolverAdapterInitializesCanonicalArena(t *testing.T) {
	solver := newTestG3Solver(t)
	adapter, err := NewArenaSolverAdapter(solver)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.CellCount() != 5 || adapter.FalseCell().ID != 0 || adapter.TrueCell().ID != 1 || adapter.BooleanSetCell().ID != 2 {
		t.Fatalf("initial solver arena = %#v", adapter.PureArena().CellMap())
	}
	if len(adapter.GetHas(adapter.BooleanSetCell())) != 2 {
		t.Fatalf("BOOLEAN children = %#v", adapter.GetHas(adapter.BooleanSetCell()))
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("initial solver arena = %v, %v", status, err)
	}
	if err := adapter.Assert(adapter.FalseCell().NameExpr()); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("FALSE cell constraint = %v, %v", status, err)
	}
}

func TestArenaSolverAdapterSynchronizesCellsEdgesAndFunctions(t *testing.T) {
	solver := newTestG3Solver(t)
	adapter, err := NewArenaSolverAdapter(solver)
	if err != nil {
		t.Fatal(err)
	}
	withSet, setCell, err := adapter.AppendTLAType(mustSet(IntT1))
	if err != nil {
		t.Fatal(err)
	}
	withElement, element, err := withSet.AppendTLAType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := withElement.AppendHas(setCell, FixedElementPointer{Cell: element})
	if err != nil {
		t.Fatal(err)
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), element.NameExpr(), setCell.NameExpr())
	if err := linked.Assert(membership); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("linked arena = %v, %v", status, err)
	}
	if adapter.CellCount() != 5 || linked.CellCount() != 7 || len(withElement.GetHas(setCell)) != 0 || len(linked.GetHas(setCell)) != 1 {
		t.Fatal("adapter updates did not preserve arena persistence")
	}
	withFunction, function, err := linked.AppendTLAType(mustFun(IntT1, IntT1))
	if err != nil {
		t.Fatal(err)
	}
	withDomain, err := withFunction.SetDomain(function, setCell)
	if err != nil {
		t.Fatal(err)
	}
	withBoth, err := withDomain.SetCodomain(function, setCell)
	if err != nil {
		t.Fatal(err)
	}
	if domain, err := withBoth.Domain(function); err != nil || !domain.Equal(setCell) {
		t.Fatalf("function domain = %v, %v", domain, err)
	}
	if _, err := withBoth.SetDomain(function, setCell); err == nil {
		t.Fatal("duplicate function domain was accepted")
	}
	if got := linked.SubgraphString(setCell); got != "5 has:(6 )" {
		t.Fatalf("subgraph = %q", got)
	}
}

func TestArenaSolverAdapterNoSMTVariantsStayExplicit(t *testing.T) {
	adapter, err := NewArenaSolverAdapter(newTestG3Solver(t))
	if err != nil {
		t.Fatal(err)
	}
	withSet, setCell := adapter.AppendCellTypeNoSMT(BasicCellType{Type: mustSet(IntT1)})
	withElement, element := withSet.AppendCellTypeNoSMT(BasicCellType{Type: IntT1})
	linked := withElement.AppendHasNoSMT(setCell, FixedElementPointer{Cell: element})
	if len(linked.GetHasPointers(setCell)) != 1 {
		t.Fatal("no-SMT edge was not retained in the pure arena")
	}
	if err := linked.Assert(MustOperExpr(Op("SET_IN"), Typed(BoolT1), element.NameExpr(), setCell.NameExpr())); err == nil {
		t.Fatal("no-SMT cell/edge unexpectedly became encodable")
	}
}

func TestArenaSolverAdapterForwardsPureArenaQueriesAndReplacesSolver(t *testing.T) {
	adapter, err := NewArenaSolverAdapter(newTestG3Solver(t))
	if err != nil {
		t.Fatal(err)
	}
	if found, err := adapter.FindCellByName("$C$1"); err != nil || !found.Equal(adapter.TrueCell()) {
		t.Fatalf("find by name = %v, %v", found, err)
	}
	if found, err := adapter.FindCellByNameExpr(NewNameExpr("$C$2", Untyped())); err != nil || !found.Equal(adapter.BooleanSetCell()) {
		t.Fatalf("find by name expression = %v, %v", found, err)
	}
	if _, err := adapter.FindCellByNameExpr(NewNameExpr("x", Untyped())); err == nil {
		t.Fatal("non-cell name expression was accepted")
	}
	if !adapter.IsLinkedViaHas(adapter.BooleanSetCell(), adapter.FalseCell()) ||
		!adapter.IsLinkedViaHas(adapter.BooleanSetCell(), adapter.TrueCell()) {
		t.Fatal("canonical BOOLEAN membership edges are missing")
	}
	boolCells := adapter.FindCellsByType(BasicCellType{Type: BoolT1})
	if len(boolCells) != 2 || boolCells[0].ID != 0 || boolCells[1].ID != 1 {
		t.Fatalf("Boolean cells = %#v", boolCells)
	}

	withFunction, function, err := adapter.AppendTLAType(mustFun(IntT1, IntT1))
	if err != nil {
		t.Fatal(err)
	}
	withDomain, err := withFunction.SetDomain(function, adapter.IntSetCell())
	if err != nil {
		t.Fatal(err)
	}
	withBoth, err := withDomain.SetCodomain(function, adapter.IntSetCell())
	if err != nil {
		t.Fatal(err)
	}
	if !withBoth.HasDomain(function) || !withBoth.HasCodomain(function) || withFunction.HasDomain(function) {
		t.Fatal("domain/codomain query did not preserve arena persistence")
	}

	replaced, err := withBoth.SetSolver(newTestG3Solver(t))
	if err != nil {
		t.Fatal(err)
	}
	if replaced.PureArena() != withBoth.PureArena() || replaced.Encoder() == withBoth.Encoder() {
		t.Fatal("solver replacement changed the arena or retained the old encoder")
	}
}
