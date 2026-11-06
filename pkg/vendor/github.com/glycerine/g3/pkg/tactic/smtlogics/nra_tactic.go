package smtlogics

// Source: src/tactic/smtlogics/nra_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewNRATactic retains the source split between a quantifier-free NLSAT
// portfolio and the NLQSAT-first quantified fallback.
func NewNRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
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
	qf := b.keep(NewQFNRATactic(m, p, config))
	nlq := b.keep(qe.NewNLQSATTacticWithParams(m, p, qe.Config{
		Context: c, Globals: g,
		Cooperation: config.ModelEvaluator.Cooperation,
		Trace:       config.ModelEvaluator.Trace,
	}))
	fallback := b.otherwise(nlq, b.keep(smtcore.NewSMTTactic(m, nil, config)))
	qfnra := b.probe(arith.NewArithmeticFragmentProbe(m.Budget(), c, g, arith.FragmentQFNRA))
	backend := b.keep(tactic.NewCondTactic(qfnra, qf, fallback))
	if b.err != nil {
		return nil, b.err
	}
	result, err = tactic.NewAndThenTactic(
		b.keep(core.MakeSimplifyTactic(m, c, g, p)),
		b.keep(core.NewPropagateValuesTactic(m, c, g, p)),
		b.keep(qe.NewLiteTactic(m, p, qe.Config{
			Context: c, Globals: g,
			Cooperation: config.ModelEvaluator.Cooperation,
			Trace:       config.ModelEvaluator.Trace,
		})),
		backend,
	)
	if b.err != nil {
		return nil, b.err
	}
	return result, err
}
