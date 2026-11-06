package sls

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices recreates the source local-search tactic with the
// destination manager, parameter owners and an independent engine.
func (t *CoreTactic) TranslateWithServices(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	p, err := t.context.TranslateTo(&t.params, c)
	if err != nil {
		return nil, err
	}
	var next *CoreTactic
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewCoreTactic(m, c, g, &p)
	if err != nil {
		return nil, err
	}
	return next, nil
}
