package tab

// Source: src/muz/tab/tab_context.cpp, index::is_subsumed and match search.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
)

type subsumptionSearch struct {
	manager        *ast.ASTManager
	goal           *groundClause
	matcher        *SemanticMatcher
	bindings       *substitution.Substitution
	conditions     []ast.ASTRef
	lite           *qe.Lite
	smt            solver.Solver
	falseLiterals  map[ast.ASTHandle]ast.ASTRef
	modelEvaluator *model.ModelEvaluatorConfig
}

func (s *subsumptionSearch) truncateConditions(n int) error {
	if n < 0 || n > len(s.conditions) {
		return ast.ErrASTDomain
	}
	for len(s.conditions) > n {
		i := len(s.conditions) - 1
		err := s.manager.DelRef(&s.conditions[i])
		if s.conditions[i] == (ast.ASTRef{}) {
			s.conditions = s.conditions[:i]
		}
		if err != nil {
			return err
		}
		if len(s.conditions) != i {
			return ast.ErrASTPending
		}
	}
	return nil
}
func (s *subsumptionSearch) matchPredicates(candidate *Clause, index int) (matched bool, err error) {
	canceled, err := s.manager.Canceled()
	if err != nil || canceled {
		return false, err
	}
	if index == len(candidate.predicates) {
		return validateSubsumption(s.manager, candidate, s.bindings, s.conditions, s.lite, s.smt, s.falseLiterals, s.modelEvaluator)
	}
	pattern, err := s.manager.RefGet(candidate.predicates[index])
	if err != nil {
		return false, err
	}
	for _, ref := range s.goal.predicates {
		canceled, err = s.manager.Canceled()
		if err != nil || canceled {
			return false, err
		}
		term, e := s.manager.RefGet(ref)
		if e != nil {
			return false, e
		}
		if e = s.bindings.PushScope(); e != nil {
			return false, e
		}
		start := len(s.conditions)
		ok, e := s.matcher.Match(pattern, term, s.bindings, &s.conditions)
		if e == nil && ok {
			ok, e = s.matchPredicates(candidate, index+1)
		}
		e = errors.Join(e, s.bindings.PopScope(1), s.truncateConditions(start))
		if e != nil {
			return false, e
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func isClauseSubsumed(query *Clause, candidates []*Clause, lite *qe.Lite, smt solver.Solver, evaluator ...*model.ModelEvaluatorConfig) (found bool, sequence uint32, err error) {
	if query == nil || query.closed || lite == nil || smt == nil {
		return false, 0, ast.ErrASTHandle
	}
	m := query.manager
	solverManager, err := smt.Manager()
	if err != nil {
		return false, 0, err
	}
	if solverManager != m {
		return false, 0, ast.ErrASTHandle
	}
	attempt := &subsumptionCleanupAttempt{manager: m, scope: subsumptionSolverScope{solver: smt}}
	defer func() {
		err = finishSubsumptionScope(attempt, err)
		if err != nil {
			found, sequence = false, 0
		}
	}()
	grounded, err := setupGroundClause(query)
	if err != nil {
		return false, 0, err
	}
	attempt.grounded = grounded
	matcher, err := NewSemanticMatcher(m)
	if err != nil {
		return false, 0, err
	}
	bindings, err := substitution.New(m)
	if err != nil {
		return false, 0, err
	}
	attempt.bindings = bindings
	if len(evaluator) > 1 {
		return false, 0, ast.ErrASTHandle
	}
	var modelEvaluator *model.ModelEvaluatorConfig
	falseLiterals := make(map[ast.ASTHandle]ast.ASTRef)
	attempt.falseLiterals = falseLiterals
	if len(evaluator) == 1 {
		modelEvaluator = evaluator[0]
	}
	search := &subsumptionSearch{manager: m, goal: grounded, matcher: matcher, bindings: bindings, lite: lite, smt: smt, falseLiterals: falseLiterals, modelEvaluator: modelEvaluator}
	attempt.search = search
	if err = attempt.scope.begin(); err != nil {
		return false, 0, err
	}
	precondition, err := m.RefGet(grounded.precondition)
	if err != nil {
		return false, 0, err
	}
	if err = smt.Assert(precondition); err != nil {
		return false, 0, err
	}
	head, err := m.RefGet(grounded.head)
	if err != nil {
		return false, 0, err
	}
	for _, candidate := range candidates {
		canceled, e := m.Canceled()
		if e != nil {
			return false, 0, e
		}
		if canceled {
			return false, 0, nil
		}
		if candidate == nil || candidate.closed || candidate.manager != m {
			return false, 0, ast.ErrASTHandle
		}
		if e = search.truncateConditions(0); e != nil {
			return false, 0, e
		}
		if e = bindings.Reset(); e != nil {
			return false, 0, e
		}
		if e = bindings.Reserve(2, candidate.numVars); e != nil {
			return false, 0, e
		}
		candidateHead, e := candidate.Head()
		if e != nil {
			return false, 0, e
		}
		ok, e := matcher.Match(head, candidateHead, bindings, &search.conditions)
		if e != nil {
			return false, 0, e
		}
		if ok {
			ok, e = search.matchPredicates(candidate, 0)
			if e != nil {
				return false, 0, e
			}
			if ok {
				return true, candidate.sequence, nil
			}
		}
	}
	return false, 0, nil
}
