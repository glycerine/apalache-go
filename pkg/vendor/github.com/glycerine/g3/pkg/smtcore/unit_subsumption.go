package smtcore

// Source: src/smt/tactic/unit_subsumption_tactic.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// UnitSubsumptionKeep returns the formulas retained by the source tactic's
// incremental unit-propagation test. The input formulas and returned handles
// are borrowed.
func UnitSubsumptionKeep(manager *ast.ASTManager, formulas []ast.ASTHandle, params *uparams.ParamsRef, config SMTSolverConfig) (_ []ast.ASTHandle, resultErr error) {
	if manager == nil || config.Params == nil {
		return nil, ast.ErrASTHandle
	}
	if len(formulas) == 0 {
		return nil, nil
	}
	solver, err := NewSMTSolver(manager, params, "", config)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, solver.Close()) }()
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	proxies := make([]ast.ASTHandle, len(formulas))
	for index, formula := range formulas {
		proxies[index], err = manager.MkFreshConst("#clause", boolean)
		if err != nil {
			return nil, err
		}
		definition, err := manager.MkIff(proxies[index], formula)
		if err != nil {
			return nil, err
		}
		if err = solver.Assert(definition); err != nil {
			return nil, err
		}
	}
	if err = solver.Push(); err != nil {
		return nil, err
	}
	keep := make([]bool, len(formulas))
	for index := range keep {
		keep[index] = true
	}
	for index := range formulas {
		if err = solver.Push(); err != nil {
			return nil, err
		}
		for candidate, proxy := range proxies {
			assertion := proxy
			if candidate == index {
				assertion, err = manager.MkNot(proxy)
			} else if !keep[candidate] {
				continue
			}
			if err == nil {
				err = solver.Assert(assertion)
			}
			if err != nil {
				return nil, err
			}
		}
		if err = solver.Push(); err != nil {
			return nil, err
		}
		keep[index] = !(solver.kernel.assertedFormulasInconsistent() || solver.kernel.internalizer.Inconsistent())
		if err = solver.Pop(2); err != nil {
			return nil, err
		}
	}
	result := make([]ast.ASTHandle, 0, len(formulas))
	for index, formula := range formulas {
		if keep[index] {
			result = append(result, formula)
		}
	}
	if err = solver.Pop(1); err != nil {
		return nil, err
	}
	return result, nil
}
