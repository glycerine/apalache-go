package opt

// Source: src/opt/sortmax.cpp mk_clause.

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) clause(literals ...ast.ASTHandle) error {
	formula, err := e.engine.owner.hold(e.engine.solver.manager.MkOrArgs(literals))
	if err != nil {
		return err
	}
	if err = e.engine.solver.backend.Assert(formula); err != nil {
		return err
	}
	e.engine.stats.Clauses++
	return nil
}
