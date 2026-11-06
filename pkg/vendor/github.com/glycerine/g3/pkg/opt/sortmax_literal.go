package opt

// Source: src/opt/sortmax.cpp mk_not/trail.

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) negate(value ast.ASTHandle) (ast.ASTHandle, error) {
	if matched, child, err := e.engine.solver.manager.IsNot(value); err != nil {
		return ast.ASTHandle{}, err
	} else if matched {
		return child, nil
	}
	return e.engine.owner.hold(e.engine.solver.manager.MkNot(value))
}
