package duality

// Source: duality_rpfp.cpp:79-94 (RedVars) and :170-182 (RedDualRela).

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

// RedVars returns owning copies of the node activation and indexed parameters.
func (g *RPFP) RedVars(node *Node) (activation *Expr, variables []*Expr, result error) {
	if err := g.open(); err != nil {
		return nil, nil, err
	}
	if node == nil || node.Owner != g || node.closed {
		return nil, nil, ErrWrapperOwner
	}
	defer func() {
		if result != nil {
			if activation != nil {
				result = errors.Join(result, activation.Close())
			}
			for _, v := range variables {
				result = errors.Join(result, v.Close())
			}
			activation = nil
			variables = nil
		}
	}()
	if g.HornClauses {
		activation, result = g.context.BoolVal(true)
	} else {
		activation, result = g.context.BoolConst("@b_" + strconv.Itoa(node.Number))
	}
	if result != nil {
		return
	}
	for _, ref := range node.Annotation.individualParams {
		var h ast.ASTHandle
		h, result = g.context.manager.RefGet(ref)
		if result != nil {
			return
		}
		var term *Expr
		term, result = NewExpr(g.context, h)
		if result != nil {
			return
		}
		indexed, err := SuffixVariable(term, node.Number)
		result = errors.Join(err, term.Close())
		if indexed != nil {
			variables = append(variables, indexed)
		}
		if result != nil {
			return
		}
	}
	return
}

// RedDualRela builds a child activation conjoined with parameter equalities.
// The source overwrites its temporary argument vector; callers own the input
// expressions here and the returned expression independently owns the result.
func (g *RPFP) RedDualRela(edge *Edge, args []*Expr, index int) (_ *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if edge == nil || edge.Owner != g || edge.closed || index < 0 || index >= len(edge.Children) {
		return nil, ErrWrapperOwner
	}
	b, vars, err := g.RedVars(edge.Children[index])
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, b.Close())
		for _, v := range vars {
			result = errors.Join(result, v.Close())
		}
	}()
	if len(args) > len(vars) {
		return nil, ast.ErrASTDomain
	}
	if len(args) == 0 {
		return b.Clone()
	}
	refs := make([]*Expr, 0, len(args))
	defer func() {
		for _, v := range refs {
			result = errors.Join(result, v.Close())
		}
	}()
	handles := make([]ast.ASTHandle, 0, len(args))
	for n, arg := range args {
		eq, err := arg.Eq(vars[n])
		if err != nil {
			return nil, err
		}
		refs = append(refs, eq)
		h, err := eq.Raw()
		if err != nil {
			return nil, err
		}
		handles = append(handles, h)
	}
	h, err := g.context.manager.MkAndArgs(handles)
	if err != nil {
		return nil, err
	}
	conjunction, err := NewExpr(g.context, h)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, conjunction.Close()) }()
	return b.And(conjunction)
}
