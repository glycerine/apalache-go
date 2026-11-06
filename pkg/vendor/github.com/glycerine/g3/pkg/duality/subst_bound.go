package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// SubstBound substitutes free de Bruijn indices relative to each binder
// depth. Replacements are simultaneous and are not shifted or recursively
// visited. Existing uninterpreted constants encountered in the input become
// background constants; inserted replacements are deliberately not declared.
// Source: duality_rpfp.cpp:3436-3476, active FromClauses consumer.
func (g *RPFP) SubstBound(replacements map[uint32]*Expr, term *Expr) (out *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := term.open(); err != nil {
		return nil, err
	}
	c := g.context
	if term.value.object.context != c {
		return nil, ErrContextMismatch
	}
	values := make(map[uint32]ast.ASTHandle, len(replacements))
	for index, replacement := range replacements {
		if err := replacement.open(); err != nil {
			return nil, err
		}
		if replacement.value.object.context != c {
			return nil, ErrContextMismatch
		}
		h, err := replacement.Raw()
		if err != nil {
			return nil, err
		}
		values[index] = h
	}
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
			if n == 0 {
				family, err := m.GetFamilyID(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if family == ast.NullFamilyID {
					wrapper, err := NewFuncDecl(c, decl)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					err = errors.Join(g.DeclareConstant(wrapper), wrapper.Close())
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
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
				if value, found := values[index-depth]; found {
					changed = value
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
	h, err := visit(root, 0)
	if err != nil {
		return nil, err
	}
	return NewExpr(c, h)
}
