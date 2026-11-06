package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, nonlinear::check and check(level).

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SearchNonlinear incrementally asserts nonlinear rule encodings and checks a
// fresh guarded query at each level. On a satisfiable result the solver keeps
// the winning model available for the source trace reconstruction phase.
func (c *Compiler) SearchNonlinear(rules *base.RuleSet, query ast.ASTHandle, solver *smtcore.SMTSolver) (status lbool.LBool, level uint32, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || solver == nil {
		return lbool.Undef, 0, ast.ErrASTHandle
	}
	m := c.manager
	if err := solver.SetProduceModels(true); err != nil {
		return lbool.Undef, 0, err
	}
	for level = 0; ; level++ {
		canceled, err := m.Canceled()
		if err != nil {
			return lbool.Undef, level, err
		}
		if canceled {
			return lbool.Undef, level, fmt.Errorf("bmc: %s", m.ResourceLimit().CancelMessage())
		}
		formulas, err := c.Compile(rules, level)
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
		status, err = c.checkNonlinearLevel(solver, query, level)
		if err != nil || status != lbool.False {
			return status, level, err
		}
		if level == ^uint32(0) {
			return lbool.Undef, level, ast.ErrASTDomain
		}
	}
}

func (c *Compiler) checkNonlinearLevel(solver *smtcore.SMTSolver, query ast.ASTHandle, level uint32) (status lbool.LBool, resultErr error) {
	m := c.manager
	var refs []ast.ASTRef
	defer func() { resultErr = disposeASTRefs(m, refs, resultErr) }()
	queryRef, err := c.CompileQuery(query, level)
	if err != nil {
		return lbool.Undef, err
	}
	refs = append(refs, queryRef)
	queryValue, err := m.RefGet(queryRef)
	if err != nil {
		return lbool.Undef, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return lbool.Undef, err
	}
	guard, err := m.MkFreshConst("q", boolean)
	if err != nil {
		return lbool.Undef, err
	}
	guardRef, err := m.NewRef(guard)
	if err != nil {
		return lbool.Undef, err
	}
	refs = append(refs, guardRef)
	implication, err := m.MkImplies(guard, queryValue)
	if err != nil {
		return lbool.Undef, err
	}
	if err = solver.AssertFormula(implication); err != nil {
		return lbool.Undef, err
	}
	return solver.CheckSat([]ast.ASTHandle{guard})
}
