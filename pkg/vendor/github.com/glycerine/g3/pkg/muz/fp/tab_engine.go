package fp

// Source: src/muz/tab/tab_context.cpp and dl_register_engine.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/tab"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) clearTabAnswer() error {
	if e.tabAnswer == (ast.ASTRef{}) {
		return nil
	}
	return e.manager.DelRef(&e.tabAnswer)
}
func (e *PropositionalEngine) closeTabSession() error {
	if e.tabSession == nil {
		return nil
	}
	if e.tabSessionBorrowed {
		e.tabSession = nil
		e.tabSessionBorrowed = false
		return nil
	}
	if err := e.tabSession.Close(); err != nil {
		return err
	}
	e.tabSession = nil
	return nil
}
func (e *PropositionalEngine) queryTabPredicate(declaration ast.ASTHandle) (status lbool.LBool, err error) {
	return e.queryTab(declaration, false)
}

func (e *PropositionalEngine) queryTabExpression(query ast.ASTHandle) (status lbool.LBool, err error) {
	e.queried = true
	e.compiled = false
	defer func() { e.recordQueryStatus(status, err) }()
	return e.queryTab(query, true)
}

func (e *PropositionalEngine) queryTab(target ast.ASTHandle, expression bool) (status lbool.LBool, err error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	if err = e.clearTabAnswer(); err != nil {
		return lbool.Undef, err
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		var valid bool
		valid, err = e.set.Finalize()
		if err != nil {
			return lbool.Undef, err
		}
		if !valid {
			return lbool.Undef, fmt.Errorf("muz/fp: negation is not stratified")
		}
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	attempt := &tabBackendAttempt{manager: e.manager, params: e.params}
	defer func() { err = errors.Join(err, attempt.dispose()) }()
	attempt.view, err = e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	fallback, err := e.manager.Symbols.Intern("weight")
	if err != nil {
		return lbool.Undef, err
	}
	selected, err := attempt.view.Symbol("tab.selection", fallback)
	if err != nil {
		return lbool.Undef, err
	}
	strategy, err := e.manager.Symbols.String(selected)
	if err != nil {
		return lbool.Undef, err
	}
	if e.tabSession == nil {
		e.tabSession, err = tab.NewSession(e.manager, strategy)
		if err != nil {
			return lbool.Undef, err
		}
	}
	attempt.rw, err = rewriter.NewTheoryRewriter(e.manager, e.params, e.globals, nil)
	if err != nil {
		return lbool.Undef, err
	}
	attempt.coop, err = cooperate.NewCooperation(e.manager.Budget())
	if err != nil {
		return lbool.Undef, err
	}
	attempt.lite, err = qe.NewLite(e.manager, nil, false, qe.Config{Context: e.params, Globals: e.globals, Cooperation: attempt.coop})
	if err != nil {
		return lbool.Undef, err
	}
	attempt.local, err = e.params.NewValue()
	if err != nil {
		return lbool.Undef, err
	}
	attempt.localOK = true
	mbqi, timeout := "mbqi", "timeout"
	if err = e.params.SetBoolName(&attempt.local, &mbqi, false); err != nil {
		return lbool.Undef, err
	}
	if err = e.params.SetUIntName(&attempt.local, &timeout, 1000); err != nil {
		return lbool.Undef, err
	}
	modelEvaluator := model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals, Memory: e.manager.NumeralService().RuntimeOwner(), Cooperation: attempt.coop}
	config := smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals, ModelEvaluator: modelEvaluator}
	attempt.query, err = smtcore.NewSMTSolver(e.manager, &attempt.local, "", config)
	if err != nil {
		return lbool.Undef, err
	}
	attempt.sub, err = smtcore.NewSMTSolver(e.manager, nil, "", config)
	if err != nil {
		return lbool.Undef, err
	}
	if err = attempt.sub.SetProduceModels(true); err != nil {
		return lbool.Undef, err
	}
	query := target
	if !expression {
		arity, e2 := e.manager.GetArity(target)
		if e2 != nil {
			return lbool.Undef, e2
		}
		args := make([]ast.ASTHandle, arity)
		for i := range args {
			sort, e2 := e.manager.GetDomain(target, uint32(i))
			if e2 != nil {
				return lbool.Undef, e2
			}
			args[i], e2 = e.manager.MkVar(arity-uint32(i)-1, sort)
			if e2 != nil {
				return lbool.Undef, e2
			}
		}
		query, err = e.manager.MkApp(target, args)
		if err != nil {
			return lbool.Undef, err
		}
	}
	attempt.queryRef, err = e.manager.NewRef(query)
	if err != nil {
		return lbool.Undef, err
	}
	var stats tab.Statistics
	if expression {
		status, e.tabAnswer, err = tab.QueryExpressionWithSessionStatistics(e.set, query, attempt.rw, attempt.lite, attempt.query, attempt.sub, e.tabSession, &stats, modelEvaluator)
	} else {
		status, e.tabAnswer, err = tab.QueryPredicateWithSession(e.set, query, attempt.rw, attempt.lite, attempt.query, attempt.sub, e.tabSession, &stats, modelEvaluator)
	}
	e.tabUnfold += stats.Unfold
	e.tabUnfoldFail += stats.UnfoldFail
	e.tabSubsumed += stats.Subsumed
	return status, err
}
