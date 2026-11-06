package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
)

// AdjustQuantifiers invokes the native qe-lite traversal only for formulas
// containing quantifiers. The engine is borrowed from the search services.
// Source: duality_rpfp.cpp:695-700; duality_wrapper.cpp:350-356.
func AdjustQuantifiers(formula *Expr, engine *qe.Lite) (out *Expr, result error) {
	h, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	c := formula.value.object.context
	m := c.manager
	has, err := m.HasQuantifiers(h)
	if err != nil {
		return nil, err
	}
	kind, err := m.GetKind(h)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTQuantifier && !has {
		return formula.Clone()
	}
	if engine == nil {
		return nil, ErrWrapperOwner
	}
	ref, err := m.NewRef(h)
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, m.DelRef(&ref))
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	proof, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&proof)) }()
	if err = engine.Rewrite(ref, proof); err != nil {
		return nil, err
	}
	changed, err := m.RefGet(ref)
	if err != nil {
		return nil, err
	}
	return NewExpr(c, changed)
}
