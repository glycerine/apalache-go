package apalache

import (
	"context"
	"reflect"
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

func TestSolverContextFactoryRejectsUnsupportedBackend(t *testing.T) {
	config := DefaultSolverConfig()
	config.SMTSolver = SMTSolverCVC5
	if solver, err := CreateSolverContext(config); err == nil {
		solver.Dispose()
		t.Fatal("CVC5 factory construction succeeded")
	}
}
