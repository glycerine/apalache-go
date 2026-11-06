package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, linear::check.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SearchLinear checks source linear encodings from level zero up to maxDepth.
// Exhaustion returns unknown; a winning model remains in the solver.
func (c *Compiler) SearchLinear(rules *base.RuleSet, query ast.ASTHandle, solver *smtcore.SMTSolver, maxDepth uint32) (status lbool.LBool, level uint32, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || solver == nil {
		return lbool.Undef, 0, ast.ErrASTHandle
	}
	linear, err := IsLinear(rules)
	if err != nil {
		return lbool.Undef, 0, err
	}
	if !linear {
		return lbool.Undef, 0, ast.ErrASTDomain
	}
	if err = solver.SetProduceModels(true); err != nil {
		return lbool.Undef, 0, err
	}
	m := c.manager
	for level = 0; level < maxDepth; level++ {
		canceled, err := m.Canceled()
		if err != nil {
			return lbool.Undef, level, err
		}
		if canceled {
			return lbool.Undef, level, fmt.Errorf("bmc: %s", m.ResourceLimit().CancelMessage())
		}
		formulas, err := c.CompileLinear(rules, level)
		if err != nil {
			return lbool.Undef, level, err
		}
		for i := range formulas {
			value, e := m.RefGet(formulas[i])
			if e == nil {
				e = solver.AssertFormula(value)
			}
			if e != nil {
				err = e
				break
			}
		}
		err = disposeASTRefs(m, formulas, err)
		if err != nil {
			return lbool.Undef, level, err
		}
		queryRef, err := c.LinearQuery(query, level)
		if err != nil {
			return lbool.Undef, level, err
		}
		queryValue, err := m.RefGet(queryRef)
		if err == nil {
			status, err = solver.CheckSat([]ast.ASTHandle{queryValue})
		}
		err = disposeASTRefs(m, []ast.ASTRef{queryRef}, err)
		if err != nil || status != lbool.False {
			return status, level, err
		}
	}
	return lbool.Undef, level, nil
}
