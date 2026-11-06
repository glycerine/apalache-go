package duality

// Source: duality_rpfp.cpp, RPFP::ComputeUnderapprox.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// ComputeUnderapprox computes child-first lower facts under the current model.
// The returned localized projection is owned by the caller.
func (g *RPFP) ComputeUnderapprox(root *Node, simplifier *interp.Manager) (out *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if simplifier == nil {
		return nil, ErrWrapperOwner
	}
	return g.computeUnderapprox(root, simplifier, make(map[*Node]bool))
}
func (g *RPFP) computeUnderapprox(node *Node, simplifier *interp.Manager, active map[*Node]bool) (out *Expr, result error) {
	if node == nil || node.closed || node.Owner != g || active[node] {
		return nil, ErrWrapperOwner
	}
	m := g.context.manager
	if node.Outgoing == nil {
		if err := node.Underapprox.SetEmpty(); err != nil {
			return nil, err
		}
		h, err := m.MkTrue()
		if err != nil {
			return nil, err
		}
		return NewExpr(g.context, h)
	}
	empty, err := g.Empty(node)
	if err != nil {
		return nil, err
	}
	if empty {
		if err := node.Underapprox.SetEmpty(); err != nil {
			return nil, err
		}
		h, err := m.MkTrue()
		if err != nil {
			return nil, err
		}
		return NewExpr(g.context, h)
	}
	active[node] = true
	defer delete(active, node)
	var terms []*Expr
	var refs []ast.ASTRef
	var replacement *Transformer
	var output *Expr
	defer func() {
		for _, term := range terms {
			result = errors.Join(result, term.Close())
		}
		for i := range refs {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
		if replacement != nil {
			result = errors.Join(result, replacement.Close())
		}
		if result != nil && output != nil {
			result = errors.Join(result, output.Close())
			out = nil
		}
	}()
	for _, child := range node.Outgoing.Children {
		term, err := g.computeUnderapprox(child, simplifier, active)
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	activation, variables, err := g.RedVars(node)
	if err != nil {
		return nil, err
	}
	terms = append(terms, activation)
	terms = append(terms, variables...)
	timer, err := startProfile(g.context, "UnderapproxFormula")
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	dual := node.Outgoing.dual
	var dualTerm *Expr
	if dual == (ast.ASTRef{}) {
		h, err := m.MkTrue()
		if err != nil {
			return nil, err
		}
		dualTerm, err = NewExpr(g.context, h)
		if err != nil {
			return nil, err
		}
	} else {
		h, err := m.RefGet(dual)
		if err != nil {
			return nil, err
		}
		dualTerm, err = NewExpr(g.context, h)
		if err != nil {
			return nil, err
		}
	}
	terms = append(terms, dualTerm)
	edge, err := g.UnderapproxFormula(dualTerm, []*Expr{activation})
	if err != nil {
		return nil, err
	}
	terms = append(terms, edge)
	if err = timer.Close(); err != nil {
		return nil, err
	}
	args := make([]ast.ASTHandle, 0, len(node.Outgoing.Children)+1)
	for i := range node.Outgoing.Children {
		h, err := terms[i].Raw()
		if err != nil {
			return nil, err
		}
		args = append(args, h)
	}
	h, err := edge.Raw()
	if err != nil {
		return nil, err
	}
	args = append(args, h)
	combined, err := m.MkAndArgs(args)
	if err != nil {
		return nil, err
	}
	combinedTerm, err := NewExpr(g.context, combined)
	if err != nil {
		return nil, err
	}
	terms = append(terms, combinedTerm)
	projected, err := g.ProjectFormula(variables, combinedTerm, simplifier)
	if err != nil {
		return nil, err
	}
	terms = append(terms, projected)
	h, err = projected.Raw()
	if err != nil {
		return nil, err
	}
	simplified, err := simplifier.Simplify(h)
	if err != nil {
		return nil, err
	}
	refs = append(refs, simplified)
	h, err = m.RefGet(simplified)
	if err != nil {
		return nil, err
	}
	substitutions := make(map[ast.ASTHandle]ast.ASTHandle)
	if len(variables) != len(node.Annotation.individualParams) {
		return nil, ast.ErrASTDomain
	}
	for i, v := range variables {
		from, err := v.Raw()
		if err != nil {
			return nil, err
		}
		to, err := m.RefGet(node.Annotation.individualParams[i])
		if err != nil {
			return nil, err
		}
		substitutions[from] = to
	}
	renamed, err := substituteExpression(g.context, h, substitutions)
	if err != nil {
		return nil, err
	}
	refs = append(refs, renamed)
	renamedHandle, err := m.RefGet(renamed)
	if err != nil {
		return nil, err
	}
	replacement, err = node.Annotation.Clone()
	if err != nil {
		return nil, err
	}
	if err := m.SetRef(replacement.formula, renamedHandle); err != nil {
		return nil, err
	}
	output, err = NewExpr(g.context, h)
	if err != nil {
		return nil, err
	}
	if err := node.Underapprox.Close(); err != nil {
		return nil, err
	}
	node.Underapprox = replacement
	replacement = nil
	return output, nil
}
