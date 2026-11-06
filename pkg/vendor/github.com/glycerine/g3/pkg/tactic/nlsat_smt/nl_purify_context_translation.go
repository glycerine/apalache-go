package nlsatsmt

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices rebuilds the cooperating solvers under target-owned services.
func (t *NLPurifyTactic) TranslateWithServices(m *ast.ASTManager, c *params.ParamsContext, g *params.GlobalParamsService, services model.ModelEvaluatorConfig) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	p, err := t.context.TranslateTo(&t.parameters, c)
	if err != nil {
		return nil, err
	}
	var next *NLPurifyTactic
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewNLPurifyTactic(m, c, g, &p, services)
	if err != nil {
		return nil, err
	}
	return next, nil
}
