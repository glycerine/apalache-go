package opt

// Source: src/opt/maxres.cpp cs_max_resolve hard correction disjunction.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) assertCorrectionBlock(correction []ast.ASTHandle) error {
	block, err := e.owner.hold(e.solver.manager.MkOrArgs(correction))
	if err != nil {
		return err
	}
	return e.solver.backend.Assert(block)
}
