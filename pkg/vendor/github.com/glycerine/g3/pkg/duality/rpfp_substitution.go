package duality

// Source: src/duality/duality_rpfp.cpp, RPFP::SubstParams and
// Z3User::SubstRec. Replacement is simultaneous and follows the source's
// quantified-body and pattern traversal without shifting bound variables.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *Transformer) substituteParams(source *Transformer) (out ast.ASTRef, resultErr error) {
	if err := t.open(); err != nil {
		return out, err
	}
	if err := source.open(); err != nil {
		return out, err
	}
	if t.context != source.context {
		return out, ErrContextMismatch
	}
	m := t.context.manager
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	changed := false
	for i, fromRef := range source.individualParams {
		if i >= len(t.individualParams) {
			break
		}
		from, err := m.RefGet(fromRef)
		if err != nil {
			return out, err
		}
		to, err := m.RefGet(t.individualParams[i])
		if err != nil {
			return out, err
		}
		if from != to {
			memo[from] = to
			changed = true
		}
	}
	root, err := m.RefGet(source.formula)
	if err != nil {
		return out, err
	}
	if !changed {
		return m.NewRef(root)
	}
	return substituteExpression(t.context, root, memo)
}

// substituteExpression follows source SubstRec without shifting bound variables.
func substituteExpression(context *Context, root ast.ASTHandle, memo map[ast.ASTHandle]ast.ASTHandle) (out ast.ASTRef, resultErr error) {
	return substituteExpressionWithDeclarations(context, root, memo, nil)
}

// Concrete function-declaration map overload used by the history proposer.
func substituteExpressionWithDeclarations(context *Context, root ast.ASTHandle, memo map[ast.ASTHandle]ast.ASTHandle, declarations map[ast.ASTHandle]ast.ASTHandle) (out ast.ASTRef, resultErr error) {
	m := context.manager
	var pins []ast.ASTRef
	defer func() {
		for i := len(pins) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&pins[i]))
		}
		if resultErr != nil && out != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&out))
		}
	}()
	var visit func(ast.ASTHandle) (ast.ASTHandle, error)
	visit = func(h ast.ASTHandle) (ast.ASTHandle, error) {
		if value, found := memo[h]; found {
			return value, nil
		}
		kind, e := m.GetKind(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result := h
		switch kind {
		case ast.ASTApp:
			decl, e := m.GetDecl(h)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			n, e := m.GetNumArgs(h)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			args := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				child, cause := m.GetArg(h, i)
				if cause != nil {
					return ast.ASTHandle{}, cause
				}
				args[i], cause = visit(child)
				if cause != nil {
					return ast.ASTHandle{}, cause
				}
			}
			if replacement, found := declarations[decl]; found {
				decl = replacement
			}
			result, e = m.MkApp(decl, args)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		case ast.ASTQuantifier:
			n, e := m.GetNumPatterns(h)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			patterns := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				pattern, cause := m.GetPattern(h, i)
				if cause != nil {
					return ast.ASTHandle{}, cause
				}
				patterns[i], cause = visit(pattern)
				if cause != nil {
					return ast.ASTHandle{}, cause
				}
			}
			body, e := m.GetQuantifierBody(h)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			body, e = visit(body)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			forall, e := m.IsForall(h)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = m.UpdateQuantifierKindPatterns(h, forall, patterns, body)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		ref, e := m.NewRef(result)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		pins = append(pins, ref)
		memo[h] = result
		return result, nil
	}
	rewritten, err := visit(root)
	if err != nil {
		return out, err
	}
	return m.NewRef(rewritten)
}

func (t *Transformer) combine(other *Transformer, conjunction bool) (resultErr error) {
	replacement, err := t.substituteParams(other)
	if err != nil {
		return err
	}
	m := t.context.manager
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&replacement)) }()
	left, err := m.RefGet(t.formula)
	if err != nil {
		return err
	}
	right, err := m.RefGet(replacement)
	if err != nil {
		return err
	}
	var combined ast.ASTHandle
	if conjunction {
		combined, err = m.MkAnd(left, right)
	} else {
		combined, err = m.MkOr(left, right)
	}
	if err != nil {
		return err
	}
	return m.SetRef(t.formula, combined)
}

// UnionWith and IntersectWith first substitute the other transformer's
// individual parameters with this transformer's corresponding parameters.
func (t *Transformer) UnionWith(other *Transformer) error     { return t.combine(other, false) }
func (t *Transformer) IntersectWith(other *Transformer) error { return t.combine(other, true) }
