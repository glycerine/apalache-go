package opt

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (d *ObjectiveDecomposer) negated(term ast.ASTHandle) (ast.ASTHandle, error) {
	if yes, inner, err := d.manager.IsNot(term); err != nil {
		return ast.ASTHandle{}, err
	} else if yes {
		return inner, nil
	}
	h, err := d.manager.MkNot(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return d.retain(h)
}
