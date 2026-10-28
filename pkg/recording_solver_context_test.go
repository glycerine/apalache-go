package apalache

import (
	"context"
	"testing"
)

func TestRecordingSolverContextExtractsOnlyActiveOperations(t *testing.T) {
	contextOne, err := NewRecordingSolverContext(nil, DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer contextOne.Dispose()
	x := NewArenaCell(0, BasicCellType{Type: IntT1})
	if err = contextOne.DeclareCell(x); err != nil {
		t.Fatal(err)
	}
	equalsOne := MustOperExpr(Op("EQ"), Typed(BoolT1), x.NameExpr(), intLiteral(1))
	if err = contextOne.Assert(equalsOne); err != nil {
		t.Fatal(err)
	}
	if err = contextOne.Push(); err != nil {
		t.Fatal(err)
	}
	equalsTwo := MustOperExpr(Op("EQ"), Typed(BoolT1), x.NameExpr(), intLiteral(2))
	if err = contextOne.Assert(equalsTwo); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := contextOne.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("pushed context = %v, %v", status, checkErr)
	}
	if err = contextOne.Pop(); err != nil || contextOne.Level() != 0 {
		t.Fatalf("pop level=%d error=%v", contextOne.Level(), err)
	}
	log := contextOne.ExtractLog()
	if log.RecursiveLength() != 2 {
		t.Fatalf("active log length = %d", log.RecursiveLength())
	}

	replayed, err := NewRecordingSolverContext(log, DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer replayed.Dispose()
	if replayed.ExtractLog().RecursiveLength() != 2 {
		t.Fatalf("replayed parent length = %d", replayed.ExtractLog().RecursiveLength())
	}
	if status, checkErr := replayed.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("replayed context = %v, %v", status, checkErr)
	}
	value, err := replayed.Evaluate(x.NameExpr())
	if err != nil {
		t.Fatal(err)
	}
	formatted, _ := FormatTLAExpression(value)
	if formatted != "1" {
		t.Fatalf("replayed x = %s", formatted)
	}
}

func TestRecordingSolverContextDifferentialParentAndMembership(t *testing.T) {
	parentContext, err := NewRecordingSolverContext(nil, DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	element := NewArenaCell(0, BasicCellType{Type: IntT1})
	set := NewArenaCell(1, BasicCellType{Type: mustSet(IntT1)})
	if err = parentContext.DeclareCell(element); err != nil {
		t.Fatal(err)
	}
	if err = parentContext.DeclareCell(set); err != nil {
		t.Fatal(err)
	}
	parent := parentContext.ExtractLog()
	_ = parentContext.Dispose()

	child, err := NewRecordingSolverContext(parent, DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer child.Dispose()
	if err = child.DeclareMembership(set, element); err != nil {
		t.Fatal(err)
	}
	if err = child.DeclareMembership(set, element); err != nil {
		t.Fatal(err)
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
	if err = child.Assert(membership); err != nil {
		t.Fatal(err)
	}
	log := child.ExtractLog()
	if log.Parent() != parent || log.RecursiveLength() != 4 || len(log.Records()) != 2 {
		t.Fatalf("differential log parent=%p length=%d records=%d", log.Parent(), log.RecursiveLength(), len(log.Records()))
	}
	if status, checkErr := child.CheckWithTimeout(context.Background(), 1); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("child check = %v, %v", status, checkErr)
	}
}

func TestRecordingSolverContextRejectsStackUnderflow(t *testing.T) {
	recording, err := NewRecordingSolverContext(nil, DefaultSolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer recording.Dispose()
	if err = recording.Pop(); err == nil {
		t.Fatal("recording context stack underflow succeeded")
	}
}
