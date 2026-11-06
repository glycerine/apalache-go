package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// GetRel returns an owning reference to the first application of an edge's
// positional relation parameter, or nil when simplification removed it.
// Source: duality_rpfp.cpp, GetRelRec/GetRel (UnhoistLoop consumer).
func (g *RPFP) GetRel(edge *Edge, child int) (*Expr, error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if edge == nil || edge.closed || edge.Owner != g || child < 0 || child >= len(edge.F.relationParams) {
		return nil, ErrWrapperOwner
	}
	m := g.context.manager
	relation, err := m.RefGet(edge.F.relationParams[child])
	if err != nil {
		return nil, err
	}
	root, err := m.RefGet(edge.F.formula)
	if err != nil {
		return nil, err
	}
	pending := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(pending) > 0 {
		h := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, err
		}
		switch kind {
		case ast.ASTApp:
			decl, err := m.GetDecl(h)
			if err != nil {
				return nil, err
			}
			if decl == relation {
				return NewExpr(g.context, h)
			}
			n, err := m.GetNumArgs(h)
			if err != nil {
				return nil, err
			}
			for i := n; i > 0; i-- {
				arg, err := m.GetArg(h, i-1)
				if err != nil {
					return nil, err
				}
				pending = append(pending, arg)
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return nil, err
			}
			pending = append(pending, body)
		}
	}
	return nil, nil
}

// GetVars returns owning references to distinct uninterpreted constants in
// depth-first source order. Bound variables and quantifier annotations are
// excluded. This is the concrete variable collection used by UnhoistPullRec.
// Source: duality_rpfp.cpp, IsVar/GetVarsRec.
func GetVars(term *Expr) (out []*Expr, result error) {
	if err := term.open(); err != nil {
		return nil, err
	}
	c := term.value.object.context
	m := c.manager
	root, err := term.Raw()
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			for _, e := range out {
				result = errors.Join(result, e.Close())
			}
			out = nil
		}
	}()
	pending := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(pending) > 0 {
		h := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		kind, err := m.GetKind(h)
		if err != nil {
			return out, err
		}
		switch kind {
		case ast.ASTApp:
			n, err := m.GetNumArgs(h)
			if err != nil {
				return out, err
			}
			family, err := m.GetFamilyID(h)
			if err != nil {
				return out, err
			}
			if n == 0 && family == ast.NullFamilyID {
				e, err := NewExpr(c, h)
				if err != nil {
					return out, err
				}
				out = append(out, e)
				continue
			}
			for i := n; i > 0; i-- {
				arg, err := m.GetArg(h, i-1)
				if err != nil {
					return out, err
				}
				pending = append(pending, arg)
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return out, err
			}
			pending = append(pending, body)
		}
	}
	return out, nil
}
