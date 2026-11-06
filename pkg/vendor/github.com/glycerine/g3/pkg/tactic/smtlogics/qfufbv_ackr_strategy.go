package smtlogics

// Source: src/tactic/smtlogics/qfufbv_tactic.cpp mk_qfufbv_preamble1/mk_qfufbv_ackr_tactic.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func NewQFUFBVAckrTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return NewQFUFBVAckrTacticWithOutput(m, p, config, os.Stdout)
}

// NewQFUFBVAckrTacticWithOutput builds the source preamble and conditional.
// Source does not update the raw backend flags here; the applying wrapper does.
func NewQFUFBVAckrTacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (result tactic.Tactic, err error) {
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
			err = &QFUFBVConstructionFailure{owner: &b, cause: err, cleanup: cleanup}
		}
		if err != nil && result != nil {
			if e := result.IncRef(); e != nil {
				err = errors.Join(err, e)
			} else {
				err = errors.Join(err, result.DecRef())
			}
			result = nil
		}
	}()
	simplify := b.params()
	if b.err == nil && p != nil {
		b.err = c.Assign(simplify, p)
	}
	for _, key := range []string{"pull_cheap_ite", "local_ctx", "ite_extra_rules", "mul2concat"} {
		b.boolean(simplify, key, true)
	}
	b.boolean(simplify, "push_ite_bv", false)
	b.uint(simplify, "local_ctx_limit", 10000000)
	if b.err != nil {
		return nil, b.err
	}
	guard := func(child tactic.Tactic) tactic.Tactic {
		if b.err == nil {
			child = b.keep(tactic.NewIfNoUnsatCoresTactic(child))
		}
		if b.err == nil {
			child = b.keep(tactic.NewIfNoProofsTactic(child))
		}
		return child
	}
	preamble := b.then(
		b.clean(b.keep(core.NewSimplifyTactic(m, c, g, nil))),
		b.keep(core.NewPropagateValuesTactic(m, c, g, nil)),
		guard(b.keep(bv.NewBVBoundCheckTactic(m, c, g, nil, config.ModelEvaluator))),
		b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator))),
		b.clean(b.keep(core.NewEliminateUnconstrainedTactic(m, c, g, nil, config.ModelEvaluator))),
		guard(b.clean(b.keep(bv.NewBVSizeReductionTactic(m, c, g, config.ModelEvaluator)))),
		b.clean(b.keep(bv.NewMaxBVSharingTactic(m, c, nil, config.ModelEvaluator.Cooperation))),
		b.using(b.clean(b.keep(core.NewSimplifyTactic(m, c, g, nil))), simplify))
	actual := b.keep(NewQFUFBVAckrCoreTactic(m, p, config, output))
	probe := b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeIsQFUFBitVector, 0))
	smt := b.keep(smtcore.NewSMTTactic(m, nil, config))
	var choice tactic.Tactic
	if b.err == nil {
		choice = b.keep(tactic.NewCondTactic(probe, actual, smt))
	}
	if b.err != nil {
		return nil, b.err
	}
	result, err = tactic.NewAndThenTactic(preamble, choice)
	return result, err
}
