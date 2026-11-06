package duality

// Source: duality_wrapper.cpp:486-517,581-582.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// LinearizeAssumptions returns owning partition terms in source postorder.
// Each partition begins with its main term followed by its extra constraints.
// The root parent uses the source SHRT_MAX sentinel.
func LinearizeAssumptions(tree *TermTree) (partitions [][]ast.ASTRef, parents []int, result error) {
	if err := tree.open(); err != nil {
		return nil, nil, err
	}
	m := tree.term.value.object.context.manager
	defer func() {
		if result != nil {
			result = errors.Join(result, CloseLinearizedAssumptions(tree.term.value.object.context, &partitions))
			parents = nil
		}
	}()
	size := tree.Number(0)
	partitions = make([][]ast.ASTRef, size)
	parents = make([]int, size)
	var visit func(*TermTree) error
	visit = func(node *TermTree) error {
		if err := node.open(); err != nil {
			return err
		}
		if node.term.value.object.context != tree.term.value.object.context {
			return ErrContextMismatch
		}
		for _, child := range node.children {
			if err := visit(child); err != nil {
				return err
			}
		}
		idx := node.number
		parents[idx] = 32767
		for _, child := range node.children {
			parents[child.number] = idx
		}
		for _, term := range append([]*Expr{node.term}, node.terms...) {
			ref, err := m.CopyRef(term.value.ref)
			if err != nil {
				return err
			}
			partitions[idx] = append(partitions[idx], ref)
		}
		return nil
	}
	result = visit(tree)
	return
}
func CloseLinearizedAssumptions(context *Context, partitions *[][]ast.ASTRef) error {
	if partitions == nil {
		return nil
	}
	for len(*partitions) > 0 {
		n := len(*partitions) - 1
		for len((*partitions)[n]) > 0 {
			last := len((*partitions)[n]) - 1
			err := context.manager.DelRef(&(*partitions)[n][last])
			if (*partitions)[n][last] == (ast.ASTRef{}) {
				(*partitions)[n] = (*partitions)[n][:last]
			}
			if err != nil {
				return err
			}
		}
		*partitions = (*partitions)[:n]
	}
	return nil
}

// UnlinearizeInterpolants reconstructs the source shape from postorder facts.
// The source omits the final root interpolant and sets that root to false.
func UnlinearizeInterpolants(shape *TermTree, facts []*Expr) (*TermTree, error) {
	if err := shape.open(); err != nil {
		return nil, err
	}
	context := shape.term.value.object.context
	size := shape.Number(0)
	if len(facts) != size-1 {
		return nil, ast.ErrASTDomain
	}
	index := 0
	var visit func(*TermTree) (out *TermTree, result error)
	visit = func(node *TermTree) (out *TermTree, result error) {
		var children []*TermTree
		defer func() {
			if result != nil {
				for _, child := range children {
					result = errors.Join(result, child.Close())
				}
			}
		}()
		for _, child := range node.children {
			rebuilt, err := visit(child)
			if err != nil {
				return nil, err
			}
			children = append(children, rebuilt)
		}
		if index < len(facts) {
			term := facts[index]
			index++
			if err := term.open(); err != nil {
				return nil, err
			}
			if term.value.object.context != context {
				return nil, ErrContextMismatch
			}
			return NewTermTree(term, children)
		}
		falsity, err := context.BoolVal(false)
		if err != nil {
			return nil, err
		}
		defer func() { result = errors.Join(result, falsity.Close()) }()
		index++
		return NewTermTree(falsity, children)
	}
	return visit(shape)
}
