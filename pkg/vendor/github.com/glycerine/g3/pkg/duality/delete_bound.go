package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// DeleteBound shifts indices at or above level down by count. Source callers
// must ensure removed variables are absent; this operation is not substitution.
// Memo keys include binder depth because shared DAG nodes may occur at several
// depths. Quantifier patterns and bodies both follow the source traversal.
// Source: duality_rpfp.cpp:3479-3517.
func DeleteBound(level, count uint32, term *Expr) (out *Expr, result error) {
	if err := term.open(); err != nil {
		return nil, err
	}
	c := term.value.object.context
	m := c.manager
	root, err := term.Raw()
	if err != nil {
		return nil, err
	}
	type key struct {
		term  ast.ASTHandle
		level uint32
	}
	memo := make(map[key]ast.ASTHandle)
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			result = errors.Join(result, m.DelRef(&pins[i]))
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	var visit func(ast.ASTHandle, uint32) (ast.ASTHandle, error)
	visit = func(h ast.ASTHandle, depth uint32) (ast.ASTHandle, error) {
		k := key{h, depth}
		if value, found := memo[k]; found {
			return value, nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		changed := h
		switch kind {
		case ast.ASTApp:
			decl, err := m.GetDecl(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			n, err := m.GetNumArgs(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				child, err := m.GetArg(h, i)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				args[i], err = visit(child, depth)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			changed, err = m.MkApp(decl, args)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		case ast.ASTQuantifier:
			bound, err := m.GetNumDeclarations(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if bound > ^uint32(0)-depth {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
			nested := depth + bound
			n, err := m.GetNumPatterns(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			patterns := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				pattern, err := m.GetPattern(h, i)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				patterns[i], err = visit(pattern, nested)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			body, err = visit(body, nested)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			forall, err := m.IsForall(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			changed, err = m.UpdateQuantifierKindPatterns(h, forall, patterns, body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		case ast.ASTVar:
			index, err := m.GetVarIndex(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if index >= depth {
				if index < count {
					return ast.ASTHandle{}, ast.ErrASTDomain
				}
				sort, err := m.GetSort(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				changed, err = m.MkVar(index-count, sort)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		}
		pin, err := m.NewRef(changed)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, pin)
		memo[k] = changed
		return changed, nil
	}
	h, err := visit(root, level)
	if err != nil {
		return nil, err
	}
	return NewExpr(c, h)
}
