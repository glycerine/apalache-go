package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp mk_purify_arith_tactic.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func MakePurifyArithTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (result tactic.Tactic, resultErr error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	var owners []tactic.Tactic
	defer func() {
		for i := len(owners) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, owners[i].DecRef())
		}
		if resultErr != nil && result != nil {
			if e := result.IncRef(); e != nil {
				resultErr = errors.Join(resultErr, e)
			} else {
				resultErr = errors.Join(resultErr, result.DecRef())
			}
			result = nil
		}
	}()
	keep := func(value tactic.Tactic, err error) (tactic.Tactic, error) {
		if err != nil {
			return nil, err
		}
		if err = value.IncRef(); err != nil {
			return nil, err
		}
		owners = append(owners, value)
		return value, nil
	}
	elim, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.Del(&elim)) }()
	if p != nil {
		if err = c.Assign(&elim, p); err != nil {
			return nil, err
		}
	}
	key := "elim_rem"
	if err = c.SetBoolName(&elim, &key, true); err != nil {
		return nil, err
	}
	skolem, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.Del(&skolem)) }()
	key = "skolemize"
	if err = c.SetBoolName(&skolem, &key, false); err != nil {
		return nil, err
	}
	snf, err := keep(core.NewSNFTactic(m, c, g, &skolem))
	if err != nil {
		return nil, err
	}
	snf, err = keep(tactic.NewUsingParamsTactic(c, snf, &skolem))
	if err != nil {
		return nil, err
	}
	simplify, err := keep(core.MakeSimplifyTactic(m, c, g, &elim))
	if err != nil {
		return nil, err
	}
	simplify, err = keep(tactic.NewUsingParamsTactic(c, simplify, &elim))
	if err != nil {
		return nil, err
	}
	purifier, err := keep(NewQFNRAPurifyTactic(m, c, g, p, services))
	if err != nil {
		return nil, err
	}
	final, err := keep(core.MakeSimplifyTactic(m, c, g, p))
	if err != nil {
		return nil, err
	}
	suffix, err := keep(tactic.NewAndThenTactic(purifier, final))
	if err != nil {
		return nil, err
	}
	suffix, err = keep(tactic.NewAndThenTactic(simplify, suffix))
	if err != nil {
		return nil, err
	}
	return tactic.NewAndThenTactic(snf, suffix)
}
