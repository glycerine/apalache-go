package tactic

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func satTacticModelConverterAssertion(ok, actions bool, line int32, condition string) error {
	if ok || !ast.ASTSourceDebug {
		return nil
	}
	if !actions {
		return sat.ErrSATAssertion
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := d.AssertionsEnabled()
	if err != nil || !enabled {
		return err
	}
	if !d.HasAssertionActions() {
		return sat.ErrSATAssertion
	}
	return diagnostic.DebugAssertionFailure(d, "src/sat/tactic/goal2sat.cpp", line, condition)
}
