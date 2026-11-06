package duality

// Source: duality_rpfp.cpp:838-851,862-889.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// SetAnnotation decodes an interpolant from indexed node vocabulary back to
// annotation parameters, replacing its activation by true.
func (g *RPFP) SetAnnotation(node *Node, term *Expr) (result error) {
	if err := g.open(); err != nil {
		return err
	}
	if node == nil || node.Owner != g || node.closed {
		return ErrWrapperOwner
	}
	if err := term.open(); err != nil {
		return err
	}
	if term.value.object.context != g.context {
		return ErrContextMismatch
	}
	b, vars, err := g.RedVars(node)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, b.Close())
		for _, v := range vars {
			result = errors.Join(result, v.Close())
		}
	}()
	m := g.context.manager
	bh, err := b.Raw()
	if err != nil {
		return err
	}
	truth, err := g.context.BoolVal(true)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, truth.Close()) }()
	th, err := truth.Raw()
	if err != nil {
		return err
	}
	memo := map[ast.ASTHandle]ast.ASTHandle{bh: th}
	for n, v := range vars {
		h, err := v.Raw()
		if err != nil {
			return err
		}
		original, err := m.RefGet(node.Annotation.individualParams[n])
		if err != nil {
			return err
		}
		memo[h] = original
	}
	root, err := term.Raw()
	if err != nil {
		return err
	}
	ref, err := substituteExpression(g.context, root, memo)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	h, err := m.RefGet(ref)
	if err != nil {
		return err
	}
	return m.SetRef(node.Annotation.formula, h)
}

func (g *RPFP) nodeFormula(node *Node, upper bool) (_ *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if node == nil || node.Owner != g || node.closed {
		return nil, ErrWrapperOwner
	}
	transformer := node.Annotation
	if upper {
		transformer = node.Bound
	}
	if !upper {
		full, err := transformer.IsFull()
		if err != nil {
			return nil, err
		}
		if full {
			return transformer.Formula()
		}
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
	if len(transformer.individualParams) < len(vars) {
		return nil, ast.ErrASTDomain
	}
	m := g.context.manager
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	for n, v := range vars {
		from, err := m.RefGet(transformer.individualParams[n])
		if err != nil {
			return nil, err
		}
		to, err := v.Raw()
		if err != nil {
			return nil, err
		}
		memo[from] = to
	}
	root, err := m.RefGet(transformer.formula)
	if err != nil {
		return nil, err
	}
	ref, err := substituteExpression(g.context, root, memo)
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
	if upper {
		negated, err := formula.Not()
		if err != nil {
			return nil, err
		}
		defer func() { result = errors.Join(result, negated.Close()) }()
		return b.And(negated)
	}
	negated, err := b.Not()
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, negated.Close()) }()
	return negated.Or(formula)
}
func (g *RPFP) GetAnnotation(node *Node) (*Expr, error) { return g.nodeFormula(node, false) }
func (g *RPFP) GetUpperBound(node *Node) (*Expr, error) { return g.nodeFormula(node, true) }
