package smtlogics

// Source: src/tactic/smtlogics/qfbv_tactic.cpp/.h.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/ast"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/aig"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type QFBVConstructionFailure struct {
	owner          *qfliaConstruction
	cause, cleanup error
}

func (e *QFBVConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *QFBVConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *QFBVConstructionFailure) OriginalCause() error { return e.cause }
func (e *QFBVConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *QFBVConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.m.NumeralService().RuntimeOwner()
}
func (e *QFBVConstructionFailure) Cleanup() error { return e.owner.close() }

func NewQFBVTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return NewQFBVTacticWithOutput(m, p, config, os.Stdout)
}

// NewQFBVTacticWithOutput borrows the session output for SAT's optional DIMACS
// publication. All strategy decisions and parameter overrides follow C++.
func NewQFBVTacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (result tactic.Tactic, err error) {
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
			err = &QFBVConstructionFailure{owner: &b, cause: err, cleanup: cleanup}
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
	copyParams := func() *uparams.ParamsRef {
		v := b.params()
		if b.err == nil && p != nil {
			b.err = c.Assign(v, p)
		}
		return v
	}
	solveP, simpP, hoistP := b.params(), copyParams(), b.params()
	localP, solverP, aigP, mainP := copyParams(), b.params(), b.params(), b.params()
	b.uint(solveP, "solve_eqs_max_occs", 2)
	for _, key := range []string{"som", "pull_cheap_ite", "local_ctx", "flat"} {
		b.boolean(simpP, key, true)
	}
	b.boolean(simpP, "push_ite_bv", false)
	b.boolean(simpP, "hoist_mul", false)
	b.uint(simpP, "local_ctx_limit", 10000000)
	b.boolean(hoistP, "hoist_mul", true)
	b.boolean(hoistP, "som", false)
	b.boolean(localP, "local_ctx", true)
	b.boolean(solverP, "preprocess", false)
	b.boolean(aigP, "aig_per_assertion", false)
	for _, key := range []string{"elim_and", "push_ite_bv", "blast_distinct"} {
		b.boolean(mainP, key, true)
	}
	if b.err != nil {
		return nil, b.err
	}
	cond := func(probe tactic.ProbeEvaluator, yes, no tactic.Tactic) tactic.Tactic {
		if b.err != nil {
			return nil
		}
		return b.keep(tactic.NewCondTactic(probe, yes, no))
	}
	probe := func(kind tactic.ProbeKind, value float64) tactic.ProbeEvaluator {
		return b.probe(tactic.NewProbe(m.Budget(), kind, value))
	}
	noProof := func(child tactic.Tactic) tactic.Tactic {
		if b.err != nil {
			return nil
		}
		return b.keep(tactic.NewIfNoProofsTactic(child))
	}
	noCore := func(child tactic.Tactic) tactic.Tactic {
		if b.err != nil {
			return nil
		}
		return b.keep(tactic.NewIfNoUnsatCoresTactic(child))
	}
	simplify := func() tactic.Tactic { return b.clean(b.keep(core.NewSimplifyTactic(m, c, g, nil))) }
	solve := func() tactic.Tactic {
		return b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator)))
	}
	smt := b.keep(smtcore.NewSMTTactic(m, nil, config))
	sat := cond(probe(tactic.ProbeProduceProofs, 0),
		b.then(simplify(), b.keep(smtcore.NewSMTTactic(m, nil, config))),
		b.clean(b.keep(sattactic.NewSATTactic(c, nil, config.ModelEvaluator, output))))
	preamble := b.then(simplify(), b.keep(core.NewPropagateValuesTactic(m, c, g, nil)), b.using(solve(), solveP),
		b.clean(b.keep(core.NewEliminateUnconstrainedTactic(m, c, g, nil, config.ModelEvaluator))),
		noProof(noCore(b.clean(b.keep(bv.NewBVSizeReductionTactic(m, c, g, config.ModelEvaluator))))),
		b.using(simplify(), simpP), b.using(simplify(), hoistP),
		b.clean(b.keep(bv.NewMaxBVSharingTactic(m, c, nil, config.ModelEvaluator.Cooperation))),
		noProof(noCore(b.keep(ackermannization.NewTactic(m, p, config.ModelEvaluator)))))
	equality := b.probe(bv.NewIsQFBVEqualityProbe(m.Budget(), c, g))
	equalityRoute := b.then(b.clean(b.keep(bv.NewBV1BlasterTactic(m, c, g, nil, config.ModelEvaluator.Cooperation))), b.using(smt, solverP))
	lowMemory := b.probe(tactic.NewBinaryProbe(m.Budget(), tactic.ProbeLess, probe(tactic.ProbeMemory, 0), probe(tactic.ProbeConstant, 300)))
	aigStage := noProof(cond(probe(tactic.ProbeProduceUnsatCores, 0),
		b.clean(b.keep(aig.NewTactic(c, nil, config.ModelEvaluator.Cooperation))),
		b.using(b.clean(b.keep(aig.NewTactic(c, nil, config.ModelEvaluator.Cooperation))), aigP)))
	postBlast := b.then(b.using(b.then(simplify(), solve()), localP), aigStage)
	var memoryStage tactic.Tactic
	if b.err == nil {
		memoryStage = b.keep(tactic.NewWhenTactic(lowMemory, postBlast))
	}
	bitRoute := b.then(b.clean(b.keep(bv.NewBitBlasterTactic(m, c, g, nil, config.ModelEvaluator.Cooperation))), memoryStage, sat)
	body := b.then(preamble, cond(equality, equalityRoute, cond(probe(tactic.ProbeIsQFBitVector, 0), bitRoute, smt)))
	if b.err != nil {
		return nil, b.err
	}
	result, err = tactic.NewUsingParamsTactic(c, body, mainP)
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
