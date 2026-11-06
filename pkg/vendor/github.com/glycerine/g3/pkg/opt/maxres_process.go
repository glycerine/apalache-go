package opt

// Source: src/opt/maxres.cpp process_unsat(exprs const&).

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) processCore(core []ast.ASTHandle) error {
	weight, err := e.splitCore(core)
	if err != nil {
		return err
	}
	if err = e.maxResolve(core, weight); err != nil {
		return err
	}
	if err = e.assertCoreBlock(core); err != nil {
		return err
	}
	e.lower.Add(&e.lower, weight)
	e.stats.Cores++
	return nil
}
