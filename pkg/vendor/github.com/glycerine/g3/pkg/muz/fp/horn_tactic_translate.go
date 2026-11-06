package fp

// Source: horn_tactic::translate.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *HornTactic) Translate(manager *ast.ASTManager) (_ tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if manager == nil || manager.Symbols != t.manager.Symbols || manager.NumeralService() != t.manager.NumeralService() {
		return nil, tacticOwnerError("translation requires shared symbol and numeral services")
	}
	translated, err := newHornTactic(manager, t.context, t.globals, t.simplify)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil && translated.paramsReady {
			resultErr = errors.Join(resultErr, translated.context.Del(&translated.params))
		}
	}()
	if err = translated.UpdateParams(&t.params); err != nil {
		return nil, err
	}
	translated.logic = t.logic
	return translated, nil
}
