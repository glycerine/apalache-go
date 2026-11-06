package duality

// Source: duality_rpfp.cpp:744-766,887-898.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// substituteHiddenRoot preserves the source's deliberately shallow hiding:
// recursive operands use SubstRec, not SubstRecHide. Quantifier patterns at
// this outer level are preserved by the body-only clone overload.
func substituteHiddenRoot(context *Context, root ast.ASTHandle, memo map[ast.ASTHandle]ast.ASTHandle, number int) (out ast.ASTRef, result error) {
	m := context.manager
	if h, ok := memo[root]; ok {
		return m.NewRef(h)
	}
	kind, err := m.GetKind(root)
	if err != nil {
		return out, err
	}
	if kind == ast.ASTApp {
		count, err := m.GetNumArgs(root)
		if err != nil {
			return out, err
		}
		family, err := m.GetFamilyID(root)
		if err != nil {
			return out, err
		}
		if count == 0 && family == ast.NullFamilyID {
			term, err := NewExpr(context, root)
			if err != nil {
				return out, err
			}
			defer func() { result = errors.Join(result, term.Close()) }()
			hidden, err := rpfpVariable(term, "@q_", number)
			if err != nil {
				return out, err
			}
			defer func() { result = errors.Join(result, hidden.Close()) }()
			h, err := hidden.Raw()
			if err != nil {
				return out, err
			}
			return m.NewRef(h)
		}
	}
	if kind == ast.ASTQuantifier {
		body, err := m.GetQuantifierBody(root)
		if err != nil {
			return out, err
		}
		ref, err := substituteExpression(context, body, memo)
		if err != nil {
			return out, err
		}
		defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
		h, err := m.RefGet(ref)
		if err != nil {
			return out, err
		}
		q, err := m.UpdateQuantifierBody(root, h)
		if err != nil {
			return out, err
		}
		return m.NewRef(q)
	}
	return substituteExpression(context, root, memo)
}
func (g *RPFP) GetUnderapprox(node *Node) (_ *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if node == nil || node.Owner != g || node.closed {
		return nil, ErrWrapperOwner
	}
	b, vars, err := g.RedVars(node)
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, b.Close())
		for _, v := range vars {
			result = errors.Join(result, v.Close())
		}
	}()
	m := g.context.manager
	if len(node.Underapprox.individualParams) < len(vars) {
		return nil, ast.ErrASTDomain
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	for n, v := range vars {
		from, err := m.RefGet(node.Underapprox.individualParams[n])
		if err != nil {
			return nil, err
		}
		to, err := v.Raw()
		if err != nil {
			return nil, err
		}
		memo[from] = to
	}
	root, err := m.RefGet(node.Underapprox.formula)
	if err != nil {
		return nil, err
	}
	ref, err := substituteHiddenRoot(g.context, root, memo, node.Number)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	h, err := m.RefGet(ref)
	if err != nil {
		return nil, err
	}
	formula, err := NewExpr(g.context, h)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	negative, err := b.Not()
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, negative.Close()) }()
	return negative.Or(formula)
}
