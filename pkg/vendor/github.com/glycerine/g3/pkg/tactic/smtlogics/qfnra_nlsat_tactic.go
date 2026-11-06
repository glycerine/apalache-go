package smtlogics

// Source: src/nlsat/tactic/qfnra_nlsat_tactic.cpp mk_qfnra_nlsat_tactic.
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

func NewQFNRANLSATTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
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
	main := copyParams()
	purify := copyParams()
	source := copyParams()
	b.boolean(main, "elim_and", true)
	b.boolean(main, "blast_distinct", true)
	b.boolean(purify, "complete", false)
	if b.err != nil {
		return nil, b.err
	}
	key := "factor"
	factorEnabled, err := c.GetBoolName(source, &key, true)
	if err != nil {
		return nil, err
	}
	simplify := func() tactic.Tactic { return b.using(b.clean(b.keep(core.NewSimplifyTactic(m, c, g, source))), main) }
	purification := func() tactic.Tactic {
		return b.using(b.keep(arith.MakePurifyArithTactic(m, c, g, source, config.ModelEvaluator)), purify)
	}
	var factor tactic.Tactic
	if factorEnabled {
		factor = b.clean(b.keep(arith.NewFactorTactic(m, c, g, source, config.ModelEvaluator)))
	} else {
		factor = b.keep(tactic.NewSkipTactic(), nil)
	}
	first := b.then(
		simplify(), purification(),
		b.keep(core.NewPropagateValuesTactic(m, c, g, source)),
		b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, source, config.ModelEvaluator))),
		purification(),
		b.clean(b.keep(core.NewEliminateUnconstrainedTactic(m, c, g, source, config.ModelEvaluator))),
		b.clean(b.keep(core.NewEliminateTermITETactic(m, c, g, source, config.ModelEvaluator))),
	)
	second := b.then(
		factor,
		b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, source, config.ModelEvaluator))),
		purification(), simplify(),
		b.clean(b.keep(core.NewTseitinCNFCoreTactic(m, c, g, source, config.ModelEvaluator))),
		simplify(),
		b.clean(b.keep(nlsattactic.NewNLSATTactic(m, c, g, source, config.ModelEvaluator))),
	)
	if b.err != nil {
		return nil, b.err
	}
	return tactic.NewAndThenTactic(first, second)
}
