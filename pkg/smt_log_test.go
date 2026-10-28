package apalache

import (
	"context"
	"reflect"
	"testing"
)

type recordingSMTLogTarget struct{ calls []string }

func (target *recordingSMTLogTarget) DeclareCell(cell ArenaCell) error {
	target.calls = append(target.calls, "cell "+cell.String())
	return nil
}
func (target *recordingSMTLogTarget) DeclareMembership(set, element ArenaCell) error {
	target.calls = append(target.calls, "in "+element.String()+" "+set.String())
	return nil
}
func (target *recordingSMTLogTarget) Assert(expression Expr) error {
	text, _ := FormatTLAExpression(expression)
	target.calls = append(target.calls, "assert "+text)
	return nil
}

func TestSMTLogReplaysParentFirstAndCopiesRecords(t *testing.T) {
	element := NewArenaCell(10, BasicCellType{Type: IntT1})
	set := NewArenaCell(11, BasicCellType{Type: mustSet(IntT1)})
	records := []SMTLogRecord{DeclareCellRecord{Cell: element}, DeclareCellRecord{Cell: set}}
	parent := NewSMTLog(nil, records)
	records[0] = nil
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
	child := NewSMTLog(parent, []SMTLogRecord{
		DeclareMembershipRecord{Set: set, Element: element},
		AssertGroundExpressionRecord{Expression: membership},
	})
	if child.RecursiveLength() != 4 || len(parent.Records()) != 2 {
		t.Fatalf("length=%d parent records=%d", child.RecursiveLength(), len(parent.Records()))
	}
	target := &recordingSMTLogTarget{}
	if err := child.Replay(target); err != nil {
		t.Fatal(err)
	}
	want := []string{"cell $C$10", "cell $C$11", "in $C$10 $C$11", `assert _si_C_si_10 \in _si_C_si_11`}
	if !reflect.DeepEqual(target.calls, want) {
		t.Fatalf("replay calls = %#v, want %#v", target.calls, want)
	}
	copy := child.Records()
	copy[0] = nil
	if child.Records()[0] == nil {
		t.Fatal("Records exposed mutable storage")
	}
}

func TestSMTLogReplaysIntoG3Encoder(t *testing.T) {
	solver := newTestG3Solver(t)
	encoder, err := NewG3GroundEncoder(solver)
	if err != nil {
		t.Fatal(err)
	}
	element := NewArenaCell(20, BasicCellType{Type: IntT1})
	set := NewArenaCell(21, BasicCellType{Type: mustSet(IntT1)})
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), element.NameExpr(), set.NameExpr())
	log := NewSMTLog(nil, []SMTLogRecord{
		DeclareCellRecord{Cell: element}, DeclareCellRecord{Cell: set},
		DeclareMembershipRecord{Set: set, Element: element},
		AssertGroundExpressionRecord{Expression: membership},
	})
	if err = log.Replay(encoder); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("replayed solver = %v, %v", status, checkErr)
	}
}

func TestSMTLogRejectsInvalidReplay(t *testing.T) {
	if err := NewSMTLog(nil, nil).Replay(nil); err == nil {
		t.Fatal("nil replay target succeeded")
	}
	if err := NewSMTLog(nil, []SMTLogRecord{AssertGroundExpressionRecord{}}).Replay(&recordingSMTLogTarget{}); err == nil {
		t.Fatal("nil assertion replay succeeded")
	}
}
