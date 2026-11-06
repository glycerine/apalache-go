package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices creates a fresh lightweight QE tactic owned by the
// destination context, using a snapshot of the source parameters.
func (t *LiteTactic) TranslateWithServices(m *ast.ASTManager, config Config) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	p, err := t.config.Context.TranslateTo(&t.params, config.Context)
	if err != nil {
		return nil, err
	}
	var next *LiteTactic
	defer func() {
		resultErr = errors.Join(resultErr, config.Context.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewLiteTactic(m, &p, config)
	if err != nil {
		return nil, err
	}
	return next, nil
}
