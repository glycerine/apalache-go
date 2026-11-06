package tab

// Source: src/muz/tab/tab_context.cpp, tab::query and imp::get_answer.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Statistics contains the tabulation engine's cumulative source counters.
type Statistics struct {
	Unfold, UnfoldFail, Subsumed uint32
}

// QueryPredicate runs tabulation for a predicate application over rules.
// It returns an owning proof for a reachable query and true for an unreachable
// query. The caller owns a nonzero answer reference and all supplied services.
func QueryPredicate(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	return QueryPredicateWithStatistics(rules, query, rw, lite, querySolver, subsumptionSolver, strategy, nil, evaluator...)
}

// QueryPredicateWithStatistics adds this query's search counters to stats.
func QueryPredicateWithStatistics(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, stats *Statistics, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	return queryPredicate(rules, query, nil, rw, lite, querySolver, subsumptionSolver, strategy, nil, stats, evaluator...)
}

// QueryPredicateWithSession runs a query using source state that persists
// across calls, including selection decay/cache and clause sequence IDs.
func QueryPredicateWithSession(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, session *Session, stats *Statistics, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	return queryPredicate(rules, query, nil, rw, lite, querySolver, subsumptionSolver, "", session, stats, evaluator...)
}

func queryPredicate(rules *base.RuleSet, query ast.ASTHandle, queryRule *base.Rule, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, session *Session, stats *Statistics, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	if rules == nil || rw == nil || rw.Manager() == nil || lite == nil || querySolver == nil || subsumptionSolver == nil {
		return lbool.Undef, answer, ast.ErrASTHandle
	}
	m := rw.Manager()
	attempt := &tabQueryAttempt{manager: m}
	defer func() { answer, err = attempt.finish(answer, err) }()
	ruleManager, err := rules.RuleManager()
	if err != nil {
		return lbool.Undef, answer, err
	}
	properties, err := base.NewRuleProperties(ruleManager)
	if err != nil {
		return lbool.Undef, answer, err
	}
	attempt.properties = properties
	if err = properties.Collect(rules); err != nil {
		return lbool.Undef, answer, err
	}
	if err = properties.CheckExistentialTail(); err != nil {
		return lbool.Undef, answer, err
	}
	if err = properties.CheckNegativePredicates(); err != nil {
		return lbool.Undef, answer, err
	}
	index, err := NewRuleIndex(rules, rw)
	if err != nil {
		return lbool.Undef, answer, err
	}
	attempt.index = index
	falsehood, err := m.MkFalse()
	if err != nil {
		return lbool.Undef, answer, err
	}
	truth, err := m.MkTrue()
	if err != nil {
		return lbool.Undef, answer, err
	}
	var goal *Clause
	if queryRule == nil {
		goal, err = NewClause(m, rw, falsehood, []ast.ASTHandle{query}, truth)
	} else {
		// Source imp::query initializes the last query rule, then replaces its
		// head with false. The query rule never enters the program index.
		goal, err = NewClauseFromRule(queryRule, rw)
	}
	attempt.goal = goal
	if err != nil {
		return lbool.Undef, answer, err
	}
	if queryRule != nil {
		if err = m.SetRef(goal.head, falsehood); err != nil {
			return lbool.Undef, answer, err
		}
	}
	var modelConfig *model.ModelEvaluatorConfig
	if len(evaluator) > 1 {
		return lbool.Undef, answer, ast.ErrASTHandle
	}
	if len(evaluator) == 1 {
		modelConfig = &evaluator[0]
	}
	search, err := newTabSearchWithSession(goal, index, rw, lite, querySolver, subsumptionSolver, strategy, session, modelConfig)
	if err != nil {
		return lbool.Undef, answer, err
	}
	attempt.goal = nil
	attempt.search = search
	defer func() {
		if stats != nil {
			stats.Unfold += search.unfold
			stats.UnfoldFail += search.unfoldFail
			stats.Subsumed += search.subsumed
		}
	}()
	if err = search.verboseInitialClause(); err != nil {
		return lbool.Undef, answer, err
	}
	if queryRule != nil && len(goal.predicates) == 0 {
		// C++ select_predicate skips the initial clause's constraint check.
		// Preserve the existing Go false/theory-only query behavior: before
		// direct initialization, the synthetic unfold ran this tautology check.
		canceled, cause := m.Canceled()
		if cause != nil {
			return lbool.Undef, answer, cause
		}
		if !canceled {
			tautology, cause := search.isTautology(goal)
			if cause != nil {
				return lbool.Undef, answer, cause
			}
			if tautology {
				answer, err = m.NewRef(truth)
				return lbool.False, answer, err
			}
		}
	}
	status, err = search.Run()
	if err != nil {
		return status, answer, err
	}
	switch status {
	case lbool.True:
		answer, err = search.Proof()
	case lbool.False:
		answer, err = m.NewRef(truth)
	}
	return status, answer, err
}
