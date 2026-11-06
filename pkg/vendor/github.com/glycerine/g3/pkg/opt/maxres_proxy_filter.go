package opt

// Source: src/opt/maxres.cpp mk_fresh_bool/filter_model_converter insertion.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) filterProxy(proxy ast.ASTHandle) error {
	declaration, err := e.solver.manager.GetDecl(proxy)
	if err != nil {
		return err
	}
	if err = e.solver.filter.Insert(declaration); err != nil {
		return err
	}
	e.stats.Auxiliaries++
	return nil
}
