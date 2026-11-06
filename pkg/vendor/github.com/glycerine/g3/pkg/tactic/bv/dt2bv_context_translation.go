package bv

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices creates an independent datatype rewriter in the target context.
func (t *DT2BVTactic) TranslateWithServices(m *ast.ASTManager, c *uparams.ParamsContext, services model.ModelEvaluatorConfig) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	p, err := t.context.TranslateTo(&t.parameters, c)
	if err != nil {
		return nil, err
	}
	var next *DT2BVTactic
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewDT2BVTactic(m, c, &p, services)
	if err != nil {
		return nil, err
	}
	return next, nil
}
