package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, operator() preparation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
)

// nlPurification owns the preparation state needed by the cooperating
// nonlinear and SMT solvers. The input goal is borrowed and updated in place.
type nlPurification struct {
	goal       *tactic.Goal
	nonlinear  *tactic.Goal
	interfaces *nlInterfaceRewriter
	predicates *nlPredicateRewriter
	extraction *solver.ClauseExtraction
	filter     *tactic.FilterModelConverter
}

// assertSMT publishes the interface definitions followed by extracted goal
// clauses, in the same order as the source preparation path.
func (p *nlPurification) assertSMT(s *smtcore.SMTSolver) error {
	if p == nil || p.goal == nil || p.interfaces == nil || p.extraction == nil || s == nil {
		return ast.ErrASTHandle
	}
	for _, group := range [][]ast.ASTHandle{p.interfaces.smtDefinitions, p.extraction.Clauses} {
		for _, formula := range group {
			if err := s.AssertFormula(formula); err != nil {
				return err
			}
		}
	}
	return p.goal.IncDepth()
}

func prepareNLPurification(g *tactic.Goal) (result *nlPurification, resultErr error) {
	if g == nil {
		return nil, ast.ErrASTHandle
	}
	if err := g.CheckOwner(); err != nil {
		return nil, err
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	if proofs {
		return nil, tactic.NewTacticException("qfufnra-purify does not support proof generation")
	}
	result = &nlPurification{goal: g}
	state := result
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, state.Close())
			result = nil
		}
	}()
	m := g.Manager()
	result.nonlinear, err = tactic.NewGoal(m, false, true, false)
	if err != nil {
		return nil, err
	}
	result.filter, err = tactic.NewFilterModel(m)
	if err != nil {
		return nil, err
	}
	result.interfaces, err = newNLInterfaceRewriter(m)
	if err != nil {
		return nil, err
	}
	if err = result.interfaces.rewriteGoal(g, result.nonlinear); err != nil {
		return nil, err
	}
	cores, err := g.UnsatCoresEnabled()
	if err != nil {
		return nil, err
	}
	if !cores {
		if err = removePureArithmetic(g, result.nonlinear); err != nil {
			return nil, err
		}
	}
	count, err := g.Size()
	if err != nil {
		return nil, err
	}
	roots := make([]ast.ASTHandle, count)
	for i := range roots {
		roots[i], err = g.Form(uint32(i))
		if err != nil {
			return nil, err
		}
	}
	polarities, err := purificationPolarities(m, roots)
	if err != nil {
		return nil, err
	}
	result.predicates, err = newNLPredicateRewriter(m, result.nonlinear, polarities)
	if err != nil {
		return nil, err
	}
	if err = result.predicates.rewriteGoal(g); err != nil {
		return nil, err
	}
	result.extraction, err = solver.ExtractClausesAndDependencies(g)
	if err != nil {
		return nil, err
	}
	for _, values := range [][]ast.ASTHandle{result.interfaces.newReals, result.predicates.newPredicates} {
		for _, value := range values {
			decl, e := m.GetDecl(value)
			if e != nil {
				return nil, e
			}
			if e = result.filter.Insert(decl); e != nil {
				return nil, e
			}
		}
	}
	for key, dependency := range result.extraction.BoolToDependency {
		if key == dependency {
			continue
		}
		decl, e := m.GetDecl(key)
		if e != nil {
			return nil, e
		}
		if e = result.filter.Insert(decl); e != nil {
			return nil, e
		}
	}
	return result, nil
}

func (p *nlPurification) Close() (result error) {
	if p == nil {
		return nil
	}
	if p.extraction != nil {
		result = errors.Join(result, p.extraction.Close())
		p.extraction = nil
	}
	if p.filter != nil {
		result = errors.Join(result, p.filter.Close())
		p.filter = nil
	}
	if p.predicates != nil {
		result = errors.Join(result, p.predicates.Close())
		p.predicates = nil
	}
	if p.interfaces != nil {
		result = errors.Join(result, p.interfaces.Close())
		p.interfaces = nil
	}
	if p.nonlinear != nil {
		result = errors.Join(result, p.nonlinear.Close())
		p.nonlinear = nil
	}
	p.goal = nil
	return result
}
