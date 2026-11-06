package opt

// Source: src/opt/maxres.cpp process_unsat(vector<exprs> const&).

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) processCoreBatch(cores [][]ast.ASTHandle) error {
	for _, core := range cores {
		if correction, pivot := e.pivotCorrection(core); pivot {
			if err := e.processCorrection(correction); err != nil {
				return err
			}
			continue
		}
		if err := e.processCore(core); err != nil {
			return err
		}
		e.clampPrimalDualLower()
		if err := e.pivotAfterCore(core); err != nil {
			return err
		}
	}
	return nil
}
