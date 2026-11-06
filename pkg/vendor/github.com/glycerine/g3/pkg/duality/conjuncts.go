package duality

// Source: duality_rpfp.cpp:2384-2406.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// CollectConjuncts preserves source traversal and multiplicity, removes true
// conjuncts, and distributes negative polarity through disjunctions.
func CollectConjuncts(term *Expr, positive bool) (out []*Expr, result error) {
	if err := term.open(); err != nil {
		return nil, err
	}
	context := term.value.object.context
	m := context.manager
	defer func() {
		if result != nil {
			for _, v := range out {
				result = errors.Join(result, v.Close())
			}
			out = nil
		}
	}()
	var visit func(ast.ASTHandle, bool) error
	visit = func(h ast.ASTHandle, pos bool) error {
		kind, err := m.GetKind(h)
		if err != nil {
			return err
		}
		if kind == ast.ASTApp {
			family, err := m.GetFamilyID(h)
			if err != nil {
				return err
			}
			if family == m.BasicFamilyID() {
				op, err := m.GetDeclKind(h)
				if err != nil {
					return err
				}
				if op == ast.ASTOpNot {
					child, err := m.GetArg(h, 0)
					if err != nil {
						return err
					}
					return visit(child, !pos)
				}
				if (pos && op == ast.ASTOpAnd) || (!pos && op == ast.ASTOpOr) {
					n, err := m.GetNumArgs(h)
					if err != nil {
						return err
					}
					for j := uint32(0); j < n; j++ {
						child, err := m.GetArg(h, j)
						if err != nil {
							return err
						}
						if err = visit(child, pos); err != nil {
							return err
						}
					}
					return nil
				}
			}
		}
		if (pos && m.IsTrue(h)) || (!pos && m.IsFalse(h)) {
			return nil
		}
		if !pos {
			negated, err := m.MkNot(h)
			if err != nil {
				return err
			}
			h = negated
		}
		v, err := NewExpr(context, h)
		if err != nil {
			return err
		}
		out = append(out, v)
		return nil
	}
	h, err := term.Raw()
	if err != nil {
		return nil, err
	}
	result = visit(h, positive)
	return
}
