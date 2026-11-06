package smtlogics

// Source: src/tactic/smtlogics/qfufnra_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/core"
	nlsatsmt "github.com/glycerine/g3/pkg/tactic/nlsat_smt"
	"github.com/glycerine/g3/pkg/util/params"
)

// NewQFUFNRATactic constructs the source preprocessing sequence and mixed
// nonlinear/SMT backend.
func NewQFUFNRATactic(m *ast.ASTManager, p *params.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, resultErr error) {
	c, g := config.Params, config.Globals
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, params.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	b := qfliaConstruction{c: c, m: m}
	defer func() {
		if cleanup := b.close(); cleanup != nil {
			resultErr = &QFLIAConstructionFailure{owner: &b, cause: resultErr, cleanup: cleanup}
		}
		if resultErr != nil && result != nil {
			if err := result.IncRef(); err != nil {
				resultErr = errors.Join(resultErr, err)
			} else {
				resultErr = errors.Join(resultErr, result.DecRef())
			}
			result = nil
		}
	}()
	copyParams := func() *params.ParamsRef {
		value := b.params()
		if b.err == nil && p != nil {
			b.err = c.Assign(value, p)
		}
		return value
	}
	main := copyParams()
	b.boolean(main, "elim_and", true)
	b.boolean(main, "blast_distinct", true)
	if b.err != nil {
		return nil, b.err
	}
	simplify := func(local *params.ParamsRef) tactic.Tactic {
		return b.using(b.clean(b.keep(core.NewSimplifyTactic(m, c, g, p))), local)
	}
	purify := b.keep(arith.MakePurifyArithTactic(m, c, g, p, config.ModelEvaluator))
	first := b.then(
		simplify(main),
		purify,
		b.keep(core.NewPropagateValuesTactic(m, c, g, p)),
		b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, p, config.ModelEvaluator))),
		b.clean(b.keep(core.NewEliminateUnconstrainedTactic(m, c, g, p, config.ModelEvaluator))),
	)
	second := b.then(
		b.clean(b.keep(core.NewEliminateTermITETactic(m, c, g, p, config.ModelEvaluator))),
		b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, p, config.ModelEvaluator))),
		simplify(main),
		b.clean(b.keep(core.NewTseitinCNFCoreTactic(m, c, g, p, config.ModelEvaluator))),
		simplify(main),
		b.keep(nlsatsmt.NewNLPurifyTactic(m, c, g, p, config.ModelEvaluator)),
	)
	if b.err != nil {
		return nil, b.err
	}
	return tactic.NewAndThenTactic(first, second)
}
