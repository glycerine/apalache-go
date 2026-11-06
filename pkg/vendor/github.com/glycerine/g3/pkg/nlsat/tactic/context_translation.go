package tactic

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices copies the tactic's parameters into the target context
// and constructs its solver state using the target's services.
func (t *NLSATTactic) TranslateWithServices(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, services model.ModelEvaluatorConfig) (result basetactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	p, err := t.context.TranslateTo(&t.params, c)
	if err != nil {
		return nil, err
	}
	var next *NLSATTactic
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewNLSATTactic(m, c, g, &p, services)
	if err != nil {
		return nil, err
	}
	return next, nil
}
