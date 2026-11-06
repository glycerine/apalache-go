package smtlogics

// Source: src/tactic/fpa/qffp_tactic.cpp/.h.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/ast"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	fpatactic "github.com/glycerine/g3/pkg/tactic/fpa"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func NewQFFPTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return NewQFFPTacticWithOutput(m, p, config, os.Stdout)
}

// NewQFFPBVTactic preserves mk_qffpbv_tactic's exact source alias.
func NewQFFPBVTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return NewQFFPBVTacticWithOutput(m, p, config, os.Stdout)
}

func NewQFFPBVTacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (tactic.Tactic, error) {
	return NewQFFPTacticWithOutput(m, p, config, output)
}

// NewQFFPTacticWithOutput retains the floating-point reduction strategy and
// supplies the command session output to its SAT backend.
func NewQFFPTacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (result tactic.Tactic, err error) {
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
	mainParams := copyParams()
	simplifyParams := copyParams()
	b.boolean(simplifyParams, "arith_lhs", true)
	b.boolean(simplifyParams, "elim_and", true)
	if b.err != nil {
		return nil, b.err
	}
	simplify := func() tactic.Tactic {
		return b.clean(b.keep(core.NewSimplifyTactic(m, c, g, p)))
	}
	noProofOrCore := func(child tactic.Tactic) tactic.Tactic {
		if b.err != nil {
			return nil
		}
		child = b.keep(tactic.NewIfNoUnsatCoresTactic(child))
		if b.err != nil {
			return nil
		}
		return b.keep(tactic.NewIfNoProofsTactic(child))
	}
	preamble := b.then(
		b.using(simplify(), simplifyParams),
		b.keep(core.NewPropagateValuesTactic(m, c, g, p)),
		b.clean(b.keep(fpatactic.NewFPA2BVTactic(m, c, g, p))),
		b.keep(core.NewPropagateValuesTactic(m, c, g, p)),
		b.using(simplify(), simplifyParams),
		noProofOrCore(b.keep(ackermannization.NewTactic(m, p, config.ModelEvaluator))),
	)
	bitBlast := b.clean(b.keep(bv.NewBitBlasterTactic(m, c, g, p, config.ModelEvaluator.Cooperation)))
	propositional := b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeIsPropositional, 0))
	proofs := b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeProduceProofs, 0))
	smt := b.keep(smtcore.NewSMTTactic(m, p, config))
	sat := b.clean(b.keep(sattactic.NewSATTactic(c, p, config.ModelEvaluator, output)))
	propositionalBackend := b.keep(tactic.NewCondTactic(proofs, smt, sat))
	nonlinearReal := b.probe(newIsFPQFNRAProbe(m.Budget()))
	nonlinearBackend := b.keep(NewQFNRATactic(m, p, config))
	fallbackSMT := b.keep(smtcore.NewSMTTactic(m, p, config))
	nonpropositionalBackend := b.keep(tactic.NewCondTactic(nonlinearReal, nonlinearBackend, fallbackSMT))
	backend := b.keep(tactic.NewCondTactic(propositional, propositionalBackend, nonpropositionalBackend))
	body := b.then(preamble, bitBlast, b.using(simplify(), simplifyParams), backend)
	if b.err != nil {
		return nil, b.err
	}
	result, err = tactic.NewUsingParamsTactic(c, body, mainParams)
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
