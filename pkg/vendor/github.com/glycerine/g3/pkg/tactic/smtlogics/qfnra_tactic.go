package smtlogics

// Source: src/tactic/smtlogics/qfnra_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewQFNRATactic retains the source route order. The available simplify and
// propagation prefix handles the polynomial CNF fragment; unsupported
// preprocessing cases fall through to the bounded and SMT routes.
func NewQFNRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
	c, g := config.Params, config.Globals
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	b := qfliaConstruction{c: c, m: m}
	defer func() {
		if cleanup := b.close(); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
		if err != nil && result != nil {
			if acquire := result.IncRef(); acquire != nil {
				err = errors.Join(err, acquire)
			} else {
				err = errors.Join(err, result.DecRef())
			}
			result = nil
		}
	}()
	copyParams := func() *uparams.ParamsRef {
		value := b.params()
		if b.err == nil && p != nil {
			b.err = c.Assign(value, p)
		}
		return value
	}
	main := copyParams()
	p0, p1, p2 := copyParams(), copyParams(), copyParams()
	for _, params := range []*uparams.ParamsRef{p0, p1, p2} {
		b.boolean(params, "elim_and", true)
		b.boolean(params, "blast_distinct", true)
	}
	b.boolean(p0, "inline_vars", true)
	b.uint(p1, "seed", 11)
	b.boolean(p1, "factor", false)
	b.uint(p2, "seed", 13)
	b.boolean(p2, "factor", false)
	factorName := "factor"
	factor0, factorErr := c.GetBoolName(p0, &factorName, true)
	if factorErr != nil && b.err == nil {
		b.err = factorErr
	}
	bounded4, bounded6 := copyParams(), copyParams()
	b.uint(bounded4, "nla2bv_max_bv_size", 4)
	b.uint(bounded6, "nla2bv_max_bv_size", 6)
	if b.err != nil {
		return nil, b.err
	}
	simplify := func(params *uparams.ParamsRef) tactic.Tactic {
		return b.using(b.clean(b.keep(core.NewSimplifyTactic(m, c, g, nil))), params)
	}
	nlsatRoute := func(params *uparams.ParamsRef, factorEnabled bool, timeout uint32, requireDecision bool) tactic.Tactic {
		purifyParams := copyParams()
		b.boolean(purifyParams, "complete", false)
		backend := b.using(b.keep(nlsattactic.NewNLSATTactic(m, c, g, params, config.ModelEvaluator)), params)
		if timeout != 0 {
			backend = b.timed(backend, timeout)
		}
		factor := b.keep(tactic.NewSkipTactic(), nil)
		if factorEnabled {
			factor = b.clean(b.keep(arith.NewFactorTactic(m, c, g, params, config.ModelEvaluator)))
		}
		route := b.then(
			simplify(params),
			b.clean(b.keep(arith.NewQFNRAPurifyTactic(m, c, g, purifyParams, config.ModelEvaluator))),
			b.keep(core.NewPropagateValuesTactic(m, c, g, nil)),
			b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, params, config.ModelEvaluator))),
			b.clean(b.keep(arith.NewQFNRAPurifyTactic(m, c, g, purifyParams, config.ModelEvaluator))),
			b.clean(b.keep(core.NewEliminateUnconstrainedTactic(m, c, g, params, config.ModelEvaluator))),
			b.clean(b.keep(core.NewEliminateTermITETactic(m, c, g, params, config.ModelEvaluator))),
			factor,
			b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, params, config.ModelEvaluator))),
			b.clean(b.keep(arith.NewQFNRAPurifyTactic(m, c, g, purifyParams, config.ModelEvaluator))),
			simplify(params),
			b.clean(b.keep(core.NewTseitinCNFCoreTactic(m, c, g, params, config.ModelEvaluator))),
			simplify(params),
			backend,
		)
		if requireDecision {
			route = b.then(route, b.keep(tactic.NewFailIfUndecidedTactic(), nil))
		}
		return route
	}
	bounded := func(params *uparams.ParamsRef) tactic.Tactic {
		return b.then(
			b.using(b.keep(arith.NewNLA2BVTactic(m, c, g, params, config.ModelEvaluator)), params),
			b.keep(smtcore.NewSMTTactic(m, nil, config)),
			b.keep(tactic.NewFailIfUndecidedTactic(), nil),
		)
	}
	timedSMT := b.then(
		b.timed(b.keep(smtcore.NewSMTTactic(m, nil, config)), 5000),
		b.keep(tactic.NewFailIfUndecidedTactic(), nil),
	)
	body := b.then(
		simplify(main),
		b.keep(core.NewPropagateValuesTactic(m, c, g, nil)),
		b.otherwise(
			nlsatRoute(p0, factor0, 5000, true),
			nlsatRoute(p1, false, 10000, true),
			bounded(bounded4),
			timedSMT,
			bounded(bounded6),
			nlsatRoute(p2, false, 0, false),
		),
	)
	if b.err != nil {
		return nil, b.err
	}
	result, err = tactic.NewUsingParamsTactic(c, body, main)
	if err != nil {
		return nil, err
	}
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	err = result.UpdateParams(p)
	return result, err
}
