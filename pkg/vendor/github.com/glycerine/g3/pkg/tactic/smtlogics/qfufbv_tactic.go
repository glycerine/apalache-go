package smtlogics

// Source: src/tactic/smtlogics/qfufbv_tactic.cpp ordinary strategy path.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type QFUFBVConstructionFailure struct {
	owner          *qfliaConstruction
	cause, cleanup error
}

func (e *QFUFBVConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *QFUFBVConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *QFUFBVConstructionFailure) OriginalCause() error { return e.cause }
func (e *QFUFBVConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *QFUFBVConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.m.NumeralService().RuntimeOwner()
}
func (e *QFUFBVConstructionFailure) Cleanup() error { return e.owner.close() }

func NewQFUFBVTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return NewQFUFBVTacticWithOutput(m, p, config, os.Stdout)
}

// NewQFUFBVTacticWithOutput preserves the ordinary source strategy. The
// separate qfufbv_ackr strategy remains a distinct port obligation.
func NewQFUFBVTacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (result tactic.Tactic, err error) {
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
	main := b.params()
	b.boolean(main, "elim_and", true)
	b.boolean(main, "blast_distinct", true)
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
		b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator))),
		b.clean(b.keep(core.NewEliminateUnconstrainedTactic(m, c, g, nil, config.ModelEvaluator))),
		guard(b.keep(core.MakeReduceArgsTactic(m, c, g, nil, config.ModelEvaluator))),
		guard(b.clean(b.keep(bv.NewBVSizeReductionTactic(m, c, g, config.ModelEvaluator)))),
		b.clean(b.keep(bv.NewMaxBVSharingTactic(m, c, nil, config.ModelEvaluator.Cooperation))))
	probe := b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeIsQFBitVector, 0))
	// Source constructs the nested QF_BV strategy with default constructor
	// parameters, then forwards p through the final outer update.
	bits := b.keep(NewQFBVTacticWithOutput(m, nil, config, output))
	smt := b.keep(smtcore.NewSMTTactic(m, nil, config))
	var choice tactic.Tactic
	if b.err == nil {
		choice = b.keep(tactic.NewCondTactic(probe, bits, smt))
	}
	body := b.then(preamble, choice)
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
