package interp

// Source checkpoint: src/interp/iz3base.cpp interpolation-tree conversion.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (b *Base) findChildren(input map[ast.ASTHandle]bool, tree ast.ASTHandle, constraints *[]ast.ASTHandle, parents *[]int, conjuncts *[]ast.ASTHandle, children *[]int, positions *[]int, merge bool) error {
	var localChildren []int
	var localConjuncts []ast.ASTHandle
	op, err := b.manager.Operator(tree)
	if err != nil {
		return err
	}
	if op == OperatorInterp {
		body, e := b.manager.Arg(tree, 0)
		if e != nil {
			return e
		}
		if e = b.findChildren(input, body, constraints, parents, &localConjuncts, &localChildren, positions, merge); e != nil {
			return e
		}
		if len(localConjuncts) == 0 {
			truth, e := b.manager.Make(OperatorTrue)
			if e != nil {
				return e
			}
			localConjuncts = append(localConjuncts, truth)
		}
		root := len(*constraints) + len(localConjuncts) - 1
		for _, formula := range localConjuncts {
			*parents = append(*parents, root)
			*constraints = append(*constraints, formula)
		}
		for _, child := range localChildren {
			if child < 0 || child >= len(*parents) {
				return ast.ErrASTDomain
			}
			(*parents)[child] = root
		}
		*children = append(*children, root)
		*positions = append(*positions, root)
		return nil
	}
	if op == OperatorAnd {
		count, e := b.manager.NumArgs(tree)
		if e != nil {
			return e
		}
		for i := uint32(0); i < count; i++ {
			child, e := b.manager.Arg(tree, i)
			if e != nil {
				return e
			}
			if e = b.findChildren(input, child, constraints, parents, &localConjuncts, &localChildren, positions, merge); e != nil {
				return e
			}
		}
	}
	if input[tree] {
		if merge && len(localConjuncts) != 0 {
			last := len(localConjuncts) - 1
			localConjuncts[last], err = b.manager.MkAnd(localConjuncts[last], tree)
			if err != nil {
				return err
			}
		} else {
			localConjuncts = append(localConjuncts, tree)
		}
	}
	*children = append(*children, localChildren...)
	*conjuncts = append(*conjuncts, localConjuncts...)
	return nil
}

func (b *Base) ToParentsRepresentation(inputConstraints []ast.ASTHandle, tree ast.ASTHandle, merge bool) (constraints []ast.ASTHandle, parents []int, theory []ast.ASTHandle, positions []int, err error) {
	if err = b.open(); err != nil {
		return nil, nil, nil, nil, err
	}
	input := make(map[ast.ASTHandle]bool, len(inputConstraints))
	for _, formula := range inputConstraints {
		input[formula] = true
	}
	op, err := b.manager.Operator(tree)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	wrapped := tree
	if op != OperatorInterp {
		wrapped, err = b.manager.Make(OperatorInterp, tree)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}
	var conjuncts []ast.ASTHandle
	var children []int
	if err = b.findChildren(input, wrapped, &constraints, &parents, &conjuncts, &children, &positions, merge); err != nil {
		return nil, nil, nil, nil, err
	}
	if op != OperatorInterp {
		if len(positions) == 0 {
			return nil, nil, nil, nil, ast.ErrASTDomain
		}
		positions = positions[:len(positions)-1]
	}
	if len(parents) == 0 {
		return nil, nil, nil, nil, ast.ErrASTDomain
	}
	parents[len(parents)-1] = ScopeTop
	used := make(map[ast.ASTHandle]bool, len(constraints))
	for _, formula := range constraints {
		used[formula] = true
	}
	for _, formula := range inputConstraints {
		if !used[formula] {
			theory = append(theory, formula)
		}
	}
	return constraints, parents, theory, positions, nil
}
