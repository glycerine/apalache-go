package opt

// Source: src/opt/sortmax.cpp fresh/filter_model_converter insertion.

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) fresh() (ast.ASTHandle, error) {
	boolean, err := e.engine.solver.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	wire, err := e.engine.owner.hold(e.engine.solver.manager.MkFreshConst("sn", boolean))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := e.engine.solver.manager.GetDecl(wire)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = e.engine.solver.filter.Insert(declaration); err != nil {
		return ast.ASTHandle{}, err
	}
	e.engine.stats.Auxiliaries++
	return wire, nil
}
