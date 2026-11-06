package opt

// Source: src/opt/maxres.cpp process_unsat hard core block.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) assertCoreBlock(core []ast.ASTHandle) error {
	conjunction, err := e.owner.hold(e.solver.manager.MkAndArgs(core))
	if err != nil {
		return err
	}
	block, err := e.owner.hold(e.solver.manager.MkNot(conjunction))
	if err != nil {
		return err
	}
	return e.solver.backend.Assert(block)
}
