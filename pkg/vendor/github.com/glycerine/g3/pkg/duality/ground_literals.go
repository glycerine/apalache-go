package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// GetGroundLitsUnderQuants collects source case-splitting candidates in DAG
// traversal order. The memo distinguishes outside/inside quantifier contexts.
// Negations are opaque literals rather than Boolean traversal nodes; quantifier
// patterns are deliberately excluded. Every returned expression owns its ref.
// Source: duality_rpfp.cpp:2529-2560.
func GetGroundLitsUnderQuants(formula *Expr) (out []*Expr, result error) {
	if err := formula.open(); err != nil {
		return nil, err
	}
	c := formula.value.object.context
	m := c.manager
	root, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	seen := [2]map[ast.ASTHandle]bool{make(map[ast.ASTHandle]bool), make(map[ast.ASTHandle]bool)}
	var owned []*Expr
	defer func() {
		if result != nil {
			for _, term := range owned {
				result = errors.Join(result, term.Close())
			}
			out = nil
		}
	}()
	var visit func(ast.ASTHandle, int) error
	visit = func(h ast.ASTHandle, under int) error {
		if seen[under][h] {
			return nil
		}
		seen[under][h] = true
		kind, err := m.GetKind(h)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			if under == 0 {
				has, err := m.HasQuantifiers(h)
				if err != nil {
					return err
				}
				if !has {
					return nil
				}
			}
			op, err := m.GetDeclKind(h)
			if err != nil {
				return err
			}
			if op == ast.ASTOpAnd || op == ast.ASTOpOr || op == ast.ASTOpImplies || op == ast.ASTOpIff {
				count, err := m.GetNumArgs(h)
				if err != nil {
					return err
				}
				for i := uint32(0); i < count; i++ {
					child, err := m.GetArg(h, i)
					if err != nil {
						return err
					}
					if err = visit(child, under); err != nil {
						return err
					}
				}
				return nil
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return err
			}
			return visit(body, 1)
		case ast.ASTVar:
			return nil
		}
		if under != 0 {
			ground, err := m.IsGround(h)
			if err != nil {
				return err
			}
			if ground {
				term, err := NewExpr(c, h)
				if err != nil {
					return err
				}
				owned = append(owned, term)
			}
		}
		return nil
	}
	if err = visit(root, 0); err != nil {
		return nil, err
	}
	return owned, nil
}
