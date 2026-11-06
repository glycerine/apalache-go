package ackermannization

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices copies the source constructor's parameter snapshot
// into an independent target-owned Ackermannization tactic.
func (t *Tactic) TranslateWithServices(m *ast.ASTManager, services model.ModelEvaluatorConfig) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil || services.Context == nil || services.Globals == nil || services.Globals.Context != services.Context {
		return nil, uparams.ErrParamsOwner
	}
	if err := services.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	p, err := t.context.TranslateTo(&t.params, services.Context)
	if err != nil {
		return nil, err
	}
	var next *Tactic
	defer func() {
		resultErr = errors.Join(resultErr, services.Context.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewTactic(m, &p, services)
	if err != nil {
		return nil, err
	}
	next.trace = t.trace
	return next, nil
}
