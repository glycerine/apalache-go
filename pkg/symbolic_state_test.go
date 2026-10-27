package apalache

import (
	"reflect"
	"testing"
)

func TestBindingMergeForgetAndShift(t *testing.T) {
	typ := BasicCellType{Type: IntT1}
	x, nextX, y, constant := NewArenaCell(0, typ), NewArenaCell(1, typ), NewArenaCell(2, typ), NewArenaCell(3, typ)
	binding := BindingOf(
		BindingEntry{Name: "x", Cell: x}, BindingEntry{Name: "x'", Cell: nextX},
		BindingEntry{Name: "y'", Cell: y}, BindingEntry{Name: "N", Cell: constant},
	)
	if !binding.Contains("x") || binding.MustGet("x'").ID != 1 || len(binding.Map()) != 4 {
		t.Fatalf("binding = %#v", binding.Map())
	}
	override := binding.Merge(BindingOf(BindingEntry{Name: "x", Cell: y}))
	if !override.MustGet("x").Equal(y) || !binding.MustGet("x").Equal(x) {
		t.Fatal("binding merge did not override persistently")
	}
	forgotPrimed := binding.ForgetPrimed()
	if forgotPrimed.Contains("x'") || !forgotPrimed.Contains("x") || !forgotPrimed.Contains("N") {
		t.Fatalf("forget primed = %#v", forgotPrimed.Map())
	}
	constants := map[string]struct{}{"N": {}}
	forgotCurrent := binding.ForgetNonPrimed(constants)
	if forgotCurrent.Contains("x") || !forgotCurrent.Contains("x'") || !forgotCurrent.Contains("N") {
		t.Fatalf("forget non-primed = %#v", forgotCurrent.Map())
	}
	shifted := binding.Shift(constants)
	if !shifted.MustGet("x").Equal(nextX) || !shifted.MustGet("y").Equal(y) || !shifted.MustGet("N").Equal(constant) || shifted.Contains("x'") {
		t.Fatalf("shifted binding = %#v", shifted.Map())
	}
}

func TestSymbolicStateCellsUpdatesAndChangedNames(t *testing.T) {
	adapter, err := NewArenaSolverAdapter(newTestG3Solver(t))
	if err != nil {
		t.Fatal(err)
	}
	withCells, cells, err := adapter.AppendCellTypes(
		BasicCellType{Type: IntT1}, BasicCellType{Type: IntT1}, BasicCellType{Type: IntT1},
	)
	if err != nil {
		t.Fatal(err)
	}
	binding := BindingOf(
		BindingEntry{Name: "x", Cell: cells[0]}, BindingEntry{Name: "x'", Cell: cells[0]},
		BindingEntry{Name: "y", Cell: cells[0]}, BindingEntry{Name: "y'", Cell: cells[1]},
		BindingEntry{Name: "z'", Cell: cells[2]},
	)
	state, err := NewSymbolicState(cells[1].NameExpr(), withCells, binding)
	if err != nil {
		t.Fatal(err)
	}
	if cell, err := state.AsCell(); err != nil || !cell.Equal(cells[1]) {
		t.Fatalf("state cell = %v, %v", cell, err)
	}
	if changed := state.ChangedNames(); !reflect.DeepEqual(changed, []string{"y'", "z'"}) {
		t.Fatalf("changed names = %v", changed)
	}
	updated, err := state.UpdateArena(func(arena *ArenaSolverAdapter) (*ArenaSolverAdapter, error) {
		result, _, err := arena.AppendTLAType(BoolT1)
		return result, err
	})
	if err != nil || updated.Arena.CellCount() != state.Arena.CellCount()+1 || updated.Expression != state.Expression {
		t.Fatalf("updated state = %#v, %v", updated, err)
	}
}

func TestInitialRewriterScope(t *testing.T) {
	scope := InitialRewriterScope()
	if scope.Arena.CellCount() != 5 || len(scope.Binding.Map()) != 0 {
		t.Fatalf("initial rewrite scope = %#v", scope)
	}
}
