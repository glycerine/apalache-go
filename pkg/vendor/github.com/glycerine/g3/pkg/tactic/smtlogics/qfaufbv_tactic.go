package smtlogics

// Source: src/tactic/smtlogics/qfaufbv_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type QFAUFBVConstructionFailure struct {
	owner          *qfliaConstruction
	cause, cleanup error
}

func (e *QFAUFBVConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *QFAUFBVConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *QFAUFBVConstructionFailure) OriginalCause() error { return e.cause }
func (e *QFAUFBVConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *QFAUFBVConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.m.NumeralService().RuntimeOwner()
}
func (e *QFAUFBVConstructionFailure) Cleanup() error { return e.owner.close() }

// NewQFAUFBVTactic preserves the source preamble, guarded size reduction and
// separate simplifier/backend parameter scopes. The caller retains the root.
func NewQFAUFBVTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
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
			err = &QFAUFBVConstructionFailure{owner: &b, cause: err, cleanup: cleanup}
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
	main, simplify, backend := b.params(), b.params(), b.params()
	if b.err == nil && p != nil {
		b.err = c.Assign(simplify, p)
	}
	for _, key := range []string{"elim_and", "sort_store"} {
		b.boolean(main, key, true)
	}
	for _, key := range []string{"som", "pull_cheap_ite", "local_ctx"} {
		b.boolean(simplify, key, true)
	}
	b.boolean(simplify, "push_ite_bv", false)
	b.uint(simplify, "local_ctx_limit", 10000000)
	// Retain the source override, even though the source array parameter
	// updater no longer reads this legacy old_simplify option.
	b.boolean(backend, "array.simplify", false)
	if b.err != nil {
		return nil, b.err
	}
	first := b.clean(b.keep(core.NewSimplifyTactic(m, c, g, nil)))
	propagate := b.keep(core.NewPropagateValuesTactic(m, c, g, nil))
	solve := b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator)))
	eliminate := b.clean(b.keep(core.NewEliminateUnconstrainedTactic(m, c, g, nil, config.ModelEvaluator)))
	reduce := b.clean(b.keep(bv.NewBVSizeReductionTactic(m, c, g, config.ModelEvaluator)))
	if b.err == nil {
		reduce = b.keep(tactic.NewIfNoUnsatCoresTactic(reduce))
	}
	if b.err == nil {
		reduce = b.keep(tactic.NewIfNoProofsTactic(reduce))
	}
	second := b.using(b.clean(b.keep(core.NewSimplifyTactic(m, c, g, nil))), simplify)
	sharing := b.clean(b.keep(bv.NewMaxBVSharingTactic(m, c, nil, config.ModelEvaluator.Cooperation)))
	preamble := b.then(first, propagate, solve, eliminate, reduce, second, sharing)
	smt := b.using(b.keep(smtcore.NewSMTTactic(m, nil, config)), backend)
	body := b.then(preamble, smt)
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
