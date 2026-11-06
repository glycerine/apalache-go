package bv

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices constructs the sharing pass with target-owned services.
func (t *MaxBVSharingTactic) TranslateWithServices(m *ast.ASTManager, c *uparams.ParamsContext, cooperation *cooperate.Cooperation) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	p, err := t.context.TranslateTo(&t.params, c)
	if err != nil {
		return nil, err
	}
	var next *MaxBVSharingTactic
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewMaxBVSharingTactic(m, c, &p, cooperation)
	if err != nil {
		return nil, err
	}
	if err = next.BindTrace(t.trace); err != nil {
		return nil, err
	}
	return next, nil
}
