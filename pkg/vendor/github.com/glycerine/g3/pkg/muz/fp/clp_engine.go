package fp

// Source: src/muz/clp/clp_context.cpp and src/muz/fp/dl_register_engine.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) queryCLPPredicate(declaration ast.ASTHandle) (result lbool.LBool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, err := e.set.Finalize()
		if err != nil {
			return lbool.Undef, err
		}
		if !valid {
			return lbool.Undef, ErrUnstratifiedNegation
		}
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	if e.predicates[declaration] == nil {
		return lbool.Undef, ErrPropositionalHorn
	}
	// CLP uses the shared default transformation pipeline, without Spacer's
	// additional slice, coalesce, and unfold stages.
	preprocessed, err := e.preprocessHornRulesWithQueryRule(declaration, nil, true, true, false, false)
	if err != nil {
		return lbool.Undef, err
	}
	attempt := &clpBackendAttempt{manager: e.manager, preprocessed: preprocessed}
	defer func() {
		resultErr = attempt.finish(resultErr)
		if resultErr != nil {
			result = lbool.Undef
		}
	}()
	queryDeclaration, err := preprocessed.rules.DerivedPredicate(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	rules, err := preprocessed.rules.PredicateRules(queryDeclaration)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		for i := len(rules) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, rules[i].Close())
		}
	}()
	if len(rules) == 0 {
		return lbool.False, nil
	}
	query, err := rules[0].Head()
	if err != nil {
		return lbool.Undef, err
	}
	attempt.local, err = e.params.NewValue()
	if err != nil {
		return lbool.Undef, err
	}
	attempt.params = e.params
	attempt.localReady = true
	mbqi := "mbqi"
	if err = e.params.SetBoolName(&attempt.local, &mbqi, false); err != nil {
		return lbool.Undef, err
	}
	timeout := "timeout"
	if err = e.params.SetUIntName(&attempt.local, &timeout, 1000); err != nil {
		return lbool.Undef, err
	}
	backend, err := smtcore.NewSMTSolver(e.manager, &attempt.local, "", smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return lbool.Undef, err
	}
	attempt.backend = backend
	engine, err := clp.NewEngine(e.manager, preprocessed.rules, backend, 20)
	if err != nil {
		return lbool.Undef, err
	}
	attempt.engine = engine
	return engine.Query(query)
}
