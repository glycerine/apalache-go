package opt

// Source: src/opt/maxres.cpp cs_max_resolve assumption definitions.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) correctionResolve(correction []ast.ASTHandle, weight *big.Rat) error {
	clauses, err := maxResCorrectionClausesForEngine(e, correction)
	if err != nil {
		return err
	}
	for _, clause := range clauses {
		proxy, err := e.freshProxy(clause, false, "a")
		if err != nil {
			return err
		}
		if err = e.addAssumption(proxy, weight); err != nil {
			return err
		}
		e.stats.CorrectionResolutions++
	}
	return nil
}
