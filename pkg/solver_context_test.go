package apalache

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type recordingSMTListener struct {
	constants []string
	cells     []int64
	asserts   []int64
}

func (listener *recordingSMTListener) OnIntroduceSMTConstant(name string) {
	listener.constants = append(listener.constants, name)
}
func (listener *recordingSMTListener) OnIntroduceCell(id int64) {
	listener.cells = append(listener.cells, id)
}
func (listener *recordingSMTListener) OnSMTAssert(_ Expr, nodes int64) {
	listener.asserts = append(listener.asserts, nodes)
}

func TestSolverContextFactoryRunsArenaExpressionsAndMetrics(t *testing.T) {
	config := DefaultSolverConfig()
	config.Debug = true
	solver, err := CreateSolverContext(config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Dispose()
	listener := &recordingSMTListener{}
	solver.SetSMTListener(listener)
	x := NewArenaCell(0, BasicCellType{Type: IntT1})
	if err = solver.DeclareCell(x); err != nil {
		t.Fatal(err)
	}
	claim := MustOperExpr(Op("EQ"), Typed(BoolT1), x.NameExpr(), intLiteral(7))
	if err = solver.Assert(claim); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("check = %v, %v", status, checkErr)
	}
	dump, err := solver.SMTLib2()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dump, "(declare-fun $C$0 () Int)") ||
		!strings.Contains(dump, "(assert (= $C$0 7))") {
		t.Fatalf("solver SMT-LIB2:\n%s", dump)
	}
	decoded, err := solver.Evaluate(x.NameExpr())
	if err != nil {
		t.Fatal(err)
	}
	formatted, _ := FormatTLAExpression(decoded)
	if formatted != "7" {
		t.Fatalf("evaluated x = %s", formatted)
	}
	wantMetrics := SolverContextMetrics{NCells: 1, NConstants: 1, NSMTExpressions: 3}
	if solver.Metrics() != wantMetrics || !reflect.DeepEqual(listener.cells, []int64{0}) ||
		!reflect.DeepEqual(listener.constants, []string{"$C$0"}) || !reflect.DeepEqual(listener.asserts, []int64{3}) {
		t.Fatalf("metrics=%#v listener=%#v", solver.Metrics(), listener)
	}
	solver.Log("hello")
	if !reflect.DeepEqual(solver.Logs(), []string{"hello"}) {
		t.Fatalf("logs = %#v", solver.Logs())
	}
}

func TestSolverContextStackTimeoutConsistencyAndMembership(t *testing.T) {
	solver, err := CreateSolverContext(DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Dispose()
	arena := EmptyPureArena()
	setType := BasicCellType{Type: mustSet(IntT1)}
	arena, element := arena.AppendCellType(BasicCellType{Type: IntT1})
	arena, set := arena.AppendCellType(setType)
	if err = solver.DeclareCell(element); err != nil {
		t.Fatal(err)
	}
	if err = solver.DeclareCell(set); err != nil {
		t.Fatal(err)
	}
	if err = solver.DeclareMembership(set, element); err != nil {
		t.Fatal(err)
	}
	if err = solver.DeclareMembership(set, element); err != nil {
		t.Fatal(err)
	}
	if err = solver.CheckConsistency(arena); err != nil {
		t.Fatal(err)
	}
	if err = solver.Push(); err != nil || solver.Level() != 1 {
		t.Fatalf("push level=%d err=%v", solver.Level(), err)
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
	if err = solver.Assert(membership); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.CheckWithTimeout(context.Background(), 1); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("timed check = %v, %v", status, checkErr)
	}
	if err = solver.Pop(); err != nil || solver.Level() != 0 {
		t.Fatalf("pop level=%d err=%v", solver.Level(), err)
	}
	if err = solver.Pop(); err == nil {
		t.Fatal("solver stack underflow succeeded")
	}
	if solver.Metrics().NConstants != 3 {
		t.Fatalf("membership metrics = %#v", solver.Metrics())
	}
}

func TestSolverContextRunsNativeArrayEncodings(t *testing.T) {
	t.Run("set arrays", func(t *testing.T) {
		config := DefaultSolverConfig()
		config.SMTEncoding = SMTEncodingArrays
		solver, err := CreateSolverContext(config)
		if err != nil {
			t.Fatal(err)
		}
		defer solver.Dispose()
		listener := &recordingSMTListener{}
		solver.SetSMTListener(listener)
		element := NewArenaCell(0, BasicCellType{Type: IntT1})
		set := NewArenaCell(1, BasicCellType{Type: mustSet(IntT1)})
		for _, cell := range []ArenaCell{element, set} {
			if err = solver.DeclareCell(cell); err != nil {
				t.Fatal(err)
			}
		}
		if err = solver.DeclareMembership(set, element); err != nil {
			t.Fatal(err)
		}
		if solver.Metrics().NConstants != 2 || !reflect.DeepEqual(listener.constants, []string{"$C$0", "$C$1"}) {
			t.Fatalf("native membership declarations should be implicit: metrics=%#v constants=%v", solver.Metrics(), listener.constants)
		}
		store := MustOperExpr(Op("Apalache!StoreInSet"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
		selected := MustOperExpr(Op("Apalache!SelectInSet"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
		if err = solver.Assert(store); err != nil {
			t.Fatal(err)
		}
		if err = solver.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), selected)); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
			t.Fatalf("stored set member = %v, %v", status, checkErr)
		}
	})

	t.Run("function arrays", func(t *testing.T) {
		config := DefaultSolverConfig()
		config.SMTEncoding = SMTEncodingFunArrays
		solver, err := CreateSolverContext(config)
		if err != nil {
			t.Fatal(err)
		}
		defer solver.Dispose()
		argument := NewArenaCell(0, BasicCellType{Type: IntT1})
		function := NewArenaCell(1, BasicCellType{Type: mustFunType(t, IntT1, BoolT1)})
		for _, cell := range []ArenaCell{argument, function} {
			if err = solver.DeclareCell(cell); err != nil {
				t.Fatal(err)
			}
		}
		store := MustOperExpr(Op("Apalache!StoreInSet"), Typed(BoolT1),
			boolExpr(true), function.NameExpr(), argument.NameExpr())
		selected := MustOperExpr(Op("Apalache!SelectInFun"), Typed(BoolT1),
			argument.NameExpr(), function.NameExpr())
		if err = solver.Assert(store); err != nil {
			t.Fatal(err)
		}
		if err = solver.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), selected)); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
			t.Fatalf("stored function result = %v, %v", status, checkErr)
		}
	})
}

func TestSolverContextFactoryRejectsUnsupportedBackend(t *testing.T) {
	config := DefaultSolverConfig()
	config.SMTSolver = SMTSolverCVC5
	if solver, err := CreateSolverContext(config); err == nil {
		solver.Dispose()
		t.Fatal("CVC5 factory construction succeeded")
	}
}

func TestSolverContextConstrainsCanonicalBooleanCells(t *testing.T) {
	solver, err := CreateSolverContext(DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Dispose()
	falseCell := NewArenaCell(0, BasicCellType{Type: BoolT1})
	trueCell := NewArenaCell(1, BasicCellType{Type: BoolT1})
	if err = solver.DeclareCell(falseCell); err != nil {
		t.Fatal(err)
	}
	if err = solver.DeclareCell(trueCell); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("canonical Boolean cells = %v, %v", status, checkErr)
	}
	for _, test := range []struct {
		cell ArenaCell
		want bool
	}{{falseCell, false}, {trueCell, true}} {
		got, evalErr := solver.Evaluate(test.cell.NameExpr())
		if evalErr != nil {
			t.Fatal(evalErr)
		}
		if value, ok := boolConstant(got); !ok || value != test.want {
			t.Fatalf("canonical cell %s = %#v", test.cell, got)
		}
	}
	if solver.Metrics() != (SolverContextMetrics{NCells: 2, NConstants: 2, NSMTExpressions: 2}) {
		t.Fatalf("canonical-cell metrics = %#v", solver.Metrics())
	}
	if err = solver.Assert(falseCell.NameExpr()); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("asserted FALSE cell = %v, %v", status, checkErr)
	}
}

func TestSolverContextRestoresScopedDeclarations(t *testing.T) {
	solver, err := CreateSolverContext(DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Dispose()
	element := NewArenaCell(5, BasicCellType{Type: IntT1})
	set := NewArenaCell(6, BasicCellType{Type: mustSet(IntT1)})
	if err = solver.DeclareCell(element); err != nil {
		t.Fatal(err)
	}
	if err = solver.DeclareCell(NewArenaCell(4, BasicCellType{Type: IntT1})); err == nil {
		t.Fatal("out-of-order cell declaration succeeded")
	}
	if err = solver.DeclareCell(set); err != nil {
		t.Fatal(err)
	}
	if err = solver.Push(); err != nil {
		t.Fatal(err)
	}
	if err = solver.DeclareMembership(set, element); err != nil {
		t.Fatal(err)
	}
	scoped := NewArenaCell(7, BasicCellType{Type: IntT1})
	if err = solver.DeclareCell(scoped); err != nil {
		t.Fatal(err)
	}
	if err = solver.Pop(); err != nil {
		t.Fatal(err)
	}
	if err = solver.DeclareMembership(set, element); err != nil {
		t.Fatalf("redeclare popped membership: %v", err)
	}
	if err = solver.DeclareCell(scoped); err != nil {
		t.Fatalf("redeclare popped cell: %v", err)
	}
}
