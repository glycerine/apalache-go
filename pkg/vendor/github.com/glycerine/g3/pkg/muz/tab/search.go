package tab

// Source: src/muz/tab/tab_context.cpp, imp search instructions and traversal.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type tabSearch struct {
	manager                        *ast.ASTManager
	rules                          *RuleIndex
	rewriter                       *rewriter.TheoryRewriter
	lite                           *qe.Lite
	querySolver, subsumptionSolver solver.Solver
	modelEvaluator                 *model.ModelEvaluatorConfig
	selection                      *predicateSelection
	selectionOwned                 bool
	session                        *Session
	stack, index, owned            []*Clause
	nextSequence                   uint32
	unfold, unfoldFail, subsumed   uint32
	closed                         bool
}

// newTabSearch takes ownership of query and borrows the rule index and services.
func newTabSearch(query *Clause, rules *RuleIndex, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, evaluator ...*model.ModelEvaluatorConfig) (search *tabSearch, err error) {
	return newTabSearchWithSession(query, rules, rw, lite, querySolver, subsumptionSolver, strategy, nil, evaluator...)
}

func newTabSearchWithSession(query *Clause, rules *RuleIndex, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, session *Session, evaluator ...*model.ModelEvaluatorConfig) (search *tabSearch, err error) {
	if query == nil || query.closed || rules == nil || rules.closed || query.manager != rules.manager || rw == nil || rw.Manager() != query.manager || lite == nil || querySolver == nil || subsumptionSolver == nil {
		return nil, ast.ErrASTHandle
	}
	for _, s := range []solver.Solver{querySolver, subsumptionSolver} {
		manager, e := s.Manager()
		if e != nil {
			return nil, e
		}
		if manager != query.manager {
			return nil, ast.ErrASTHandle
		}
	}
	var selection *predicateSelection
	selectionOwned := session == nil
	if session != nil {
		if err = session.open(query.manager); err != nil {
			return nil, err
		}
		selection = session.selection
	} else {
		selection, err = newPredicateSelection(query.manager, strategy)
		if err != nil {
			return nil, err
		}
	}
	if err = selection.Init(rules); err != nil {
		if selectionOwned {
			return nil, errors.Join(err, selection.Close())
		}
		return nil, err
	}
	if len(evaluator) > 1 {
		if selectionOwned {
			return nil, errors.Join(ast.ErrASTHandle, selection.Close())
		}
		return nil, ast.ErrASTHandle
	}
	var modelEvaluator *model.ModelEvaluatorConfig
	if len(evaluator) == 1 {
		modelEvaluator = evaluator[0]
	}
	search = &tabSearch{manager: query.manager, rules: rules, rewriter: rw, lite: lite, querySolver: querySolver, subsumptionSolver: subsumptionSolver, modelEvaluator: modelEvaluator, selection: selection, selectionOwned: selectionOwned, session: session}
	if session != nil {
		search.nextSequence = session.nextSequence
	}
	search.appendClause(query)
	return search, nil
}
func (s *tabSearch) appendClause(c *Clause) {
	c.index = uint32(len(s.stack))
	c.sequence = s.nextSequence
	s.nextSequence++
	s.stack = append(s.stack, c)
	s.owned = append(s.owned, c)
}
func (s *tabSearch) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if s.selection != nil && s.selectionOwned {
		if err := s.selection.Close(); err != nil {
			return err
		}
		s.selection = nil
	}
	if s.session != nil {
		s.session.nextSequence = s.nextSequence
		s.session = nil
	}
	if err := s.cleanupClauses(); err != nil {
		return err
	}
	s.closed = true
	return nil
}

// cleanupClauses implements imp::cleanup: the active query path and every
// derived clause are released immediately on cancellation, while the reusable
// selection and rule-index services remain borrowed until Close.
func (s *tabSearch) cleanupClauses() error {
	for len(s.owned) > 0 {
		i := len(s.owned) - 1
		if err := s.owned[i].Close(); err != nil {
			return err
		}
		s.owned = s.owned[:i]
	}
	s.stack, s.index = nil, nil
	return nil
}
func (s *tabSearch) isTautology(c *Clause) (tautology bool, err error) {
	attempt := &tautologyCheckAttempt{manager: s.manager, solver: s.querySolver}
	defer func() { tautology, err = finishTautologyCheck(attempt, tautology, err) }()
	formula, err := c.Formula()
	if err != nil {
		return false, err
	}
	attempt.formula = formula
	h, err := s.manager.RefGet(formula)
	if err != nil {
		return false, err
	}
	negated, err := s.manager.MkNot(h)
	if err != nil {
		return false, err
	}
	if err = s.manager.SetRef(formula, negated); err != nil {
		return false, err
	}
	attempt.target, err = s.querySolver.ScopeLevel()
	if err != nil {
		return false, err
	}
	// Push may publish its scope before reporting a cleanup refusal. Mark the
	// attempt first so the finalizer reconciles the observed solver depth.
	attempt.pushed = true
	if err = s.querySolver.Push(); err != nil {
		return false, err
	}
	if err = s.querySolver.Assert(negated); err != nil {
		return false, err
	}
	status, err := s.querySolver.CheckSat(nil)
	if err != nil {
		return false, err
	}
	return status == lbool.False, nil
}
func (s *tabSearch) Run() (lbool.LBool, error) {
	if s == nil || s.closed || s.manager == nil {
		return lbool.Undef, ast.ErrASTHandle
	}
	selectPredicate := true
	backtrack, complete := false, false
	for {
		canceled, err := s.manager.Canceled()
		if err != nil {
			return lbool.Undef, err
		}
		if canceled {
			return lbool.Undef, s.cleanupClauses()
		}
		// Source run checks cancellation before interpreting SATISFIABLE or
		// UNSATISFIABLE, including after the empty-clause diagnostic.
		if complete {
			return lbool.True, nil
		}
		if len(s.stack) == 0 {
			return lbool.False, nil
		}
		if backtrack {
			s.stack = s.stack[:len(s.stack)-1]
			backtrack = false
			continue
		}
		current := s.stack[len(s.stack)-1]
		if selectPredicate {
			if len(current.predicates) == 0 {
				if err = s.verboseEmptyClause(current); err != nil {
					return lbool.Undef, err
				}
				complete = true
				continue
			}
			current.predicateIndex, err = s.selection.Select(current)
			if err != nil {
				return lbool.Undef, err
			}
			if err = s.verboseSelectedPredicate(current); err != nil {
				return lbool.Undef, err
			}
			selectPredicate = false
			continue
		}
		current.nextRule++
		predicate, err := s.manager.RefGet(current.predicates[current.predicateIndex])
		if err != nil {
			return lbool.Undef, err
		}
		decl, err := s.manager.GetDecl(predicate)
		if err != nil {
			return lbool.Undef, err
		}
		count, err := s.rules.Count(decl)
		if err != nil {
			return lbool.Undef, err
		}
		if uint64(current.nextRule) >= uint64(count) {
			// BACKTRACK is a separate source instruction, with its own
			// cancellation boundary before retiring the active stack entry.
			backtrack = true
			continue
		}
		rule, err := s.rules.Rule(decl, int(current.nextRule))
		if err != nil {
			return lbool.Undef, err
		}
		unfolded, matched, err := unfoldClauseExpressions(current, current.predicateIndex, rule)
		if err != nil {
			return lbool.Undef, err
		}
		if !matched {
			s.unfoldFail++
			continue
		}
		child, matched, finishErr := finalizeUnfoldedClause(unfolded, s.rewriter, s.lite)
		finishErr = errors.Join(finishErr, unfolded.Close())
		if finishErr != nil {
			return lbool.Undef, finishErr
		}
		if !matched {
			s.unfoldFail++
			continue
		}
		tautology, e := s.isTautology(child)
		if e != nil {
			return lbool.Undef, errors.Join(e, child.Close())
		}
		if tautology {
			s.unfoldFail++
			if e = child.Close(); e != nil {
				return lbool.Undef, e
			}
			continue
		}
		child.index = uint32(len(s.stack))
		child.sequence = s.nextSequence
		s.nextSequence++
		found, _, e := isClauseSubsumed(child, s.index, s.lite, s.subsumptionSolver, s.modelEvaluator)
		if e != nil {
			return lbool.Undef, errors.Join(e, child.Close())
		}
		if found {
			s.subsumed++
			if e = child.Close(); e != nil {
				return lbool.Undef, e
			}
			continue
		}
		child.parentIndex = current.index
		child.parentRule = current.nextRule
		s.stack = append(s.stack, child)
		s.owned = append(s.owned, child)
		s.index = append(s.index, child)
		s.unfold++
		selectPredicate = true
	}
}
