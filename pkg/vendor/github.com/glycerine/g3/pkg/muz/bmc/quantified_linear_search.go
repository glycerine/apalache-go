package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::check.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SearchQuantifiedLinear checks increasing index widths beginning at four.
// A satisfiable result returns an owning model reference, which remains valid
// after the temporary solver scope is popped and must be released by DecRef.
func (c *Compiler) SearchQuantifiedLinear(rules *base.RuleSet, query ast.ASTHandle, solver *smtcore.SMTSolver) (status lbool.LBool, width uint32, winning *model.ModelCore, resultErr error) {
	return c.searchQuantifiedLinear(rules, query, solver, nil)
}

// SearchQuantifiedLinearInScope evaluates the winning model before its solver
// scope is popped, matching qlinear::check. The returned model remains owned
// by the caller, including when the callback succeeds.
func (c *Compiler) SearchQuantifiedLinearInScope(rules *base.RuleSet, query ast.ASTHandle, solver *smtcore.SMTSolver, consume func(uint32, *model.ModelCore) error) (lbool.LBool, uint32, *model.ModelCore, error) {
	if consume == nil {
		return lbool.Undef, 0, nil, ast.ErrASTHandle
	}
	return c.searchQuantifiedLinear(rules, query, solver, consume)
}

func (c *Compiler) searchQuantifiedLinear(rules *base.RuleSet, query ast.ASTHandle, solver *smtcore.SMTSolver, consume func(uint32, *model.ModelCore) error) (status lbool.LBool, width uint32, winning *model.ModelCore, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || solver == nil {
		return lbool.Undef, 0, nil, ast.ErrASTHandle
	}
	if err := solver.SetProduceModels(true); err != nil {
		return lbool.Undef, 0, nil, err
	}
	for width = 4; ; width++ {
		canceled, err := c.manager.Canceled()
		if err != nil {
			return lbool.Undef, width, nil, err
		}
		if canceled {
			return lbool.Undef, width, nil, fmt.Errorf("bmc: %s", c.manager.ResourceLimit().CancelMessage())
		}
		status, winning, err = c.checkQuantifiedLinearWidth(rules, query, solver, width, consume)
		if err != nil || status != lbool.False {
			return status, width, winning, err
		}
		if width == ^uint32(0) {
			return lbool.Undef, width, nil, ast.ErrASTDomain
		}
	}
}

func (c *Compiler) checkQuantifiedLinearWidth(rules *base.RuleSet, query ast.ASTHandle, solver *smtcore.SMTSolver, width uint32, consume func(uint32, *model.ModelCore) error) (status lbool.LBool, winning *model.ModelCore, resultErr error) {
	attempt := &quantifiedLinearSearchAttempt{manager: c.manager, solver: solver}
	defer func() {
		winning, resultErr = finishQuantifiedLinearSearch(attempt, resultErr)
		if resultErr != nil {
			status = lbool.Undef
		}
	}()
	if err := attempt.begin(); err != nil {
		return lbool.Undef, nil, err
	}
	formulas, err := c.CompileQuantifiedLinear(rules, width)
	if err != nil {
		return lbool.Undef, nil, err
	}
	for i := range formulas {
		value, cause := c.manager.RefGet(formulas[i])
		if cause == nil {
			cause = solver.AssertFormula(value)
		}
		if cause != nil {
			err = cause
			break
		}
	}
	// qlinear::check owns the complete compiled formula vector until it has
	// been asserted. Preserve the reverse cleanup frontier when native AST
	// destruction refuses instead of losing the remaining formula roots.
	err = disposeASTRefs(c.manager, formulas, err)
	if err != nil {
		return lbool.Undef, nil, err
	}
	// qlinear::check checkpoints after compiling/asserting one width and
	// before publishing its indexed query. The outer pre-width checkpoint is
	// not a substitute: compilation may consume the remaining resource limit.
	canceled, err := c.manager.Canceled()
	if err != nil {
		return lbool.Undef, nil, err
	}
	if canceled {
		return lbool.Undef, nil, fmt.Errorf("bmc: %s", c.manager.ResourceLimit().CancelMessage())
	}
	q, err := newQuantifiedLinear(c.manager, width)
	if err != nil {
		return lbool.Undef, nil, err
	}
	attempt.terms = q
	name, err := c.manager.Symbols.Intern("T")
	if err != nil {
		return lbool.Undef, nil, err
	}
	index, err := q.hold(c.manager.MkConst(name, q.index))
	if err != nil {
		return lbool.Undef, nil, err
	}
	goal, err := q.predicate(query, index)
	if err != nil {
		return lbool.Undef, nil, err
	}
	if err = solver.AssertFormula(goal); err != nil {
		return lbool.Undef, nil, err
	}
	status, err = solver.CheckSat(nil)
	if err != nil || status != lbool.True {
		return status, nil, err
	}
	attempt.winning, err = solver.AcquireModel()
	if err != nil || attempt.winning == nil {
		return lbool.Undef, nil, errors.Join(err, ast.ErrASTDomain)
	}
	if consume != nil {
		if err = consume(width, attempt.winning); err != nil {
			return lbool.Undef, nil, err
		}
	}
	return status, nil, nil
}
