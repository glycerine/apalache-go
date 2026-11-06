package bv

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices recreates the blaster with the target parameter owner.
func (t *BV1BlasterTactic) TranslateWithServices(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, cooperation *cooperate.Cooperation) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	p, err := t.context.TranslateTo(&t.params, c)
	if err != nil {
		return nil, err
	}
	var next *BV1BlasterTactic
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewBV1BlasterTactic(m, c, g, &p, cooperation)
	if err != nil {
		return nil, err
	}
	next.trace = t.trace
	return next, nil
}
