package proofs

// Source: src/ast/proofs/proof_utils.cpp:27-72;
// src/ast/proofs/proof_utils.h:28-41.

import "github.com/glycerine/g3/pkg/ast"

// ProofPostOrder iterates over a borrowed proof DAG in the same iterative
// parent-before-child order as proof_post_order. The caller must keep the root
// proof alive for the lifetime of the iterator.
type ProofPostOrder struct {
	self    *ProofPostOrder
	manager *ast.ASTManager
	todo    []ast.ASTHandle
	visited map[ast.ASTHandle]struct{}
}

func NewProofPostOrder(root ast.ASTHandle, m *ast.ASTManager) (*ProofPostOrder, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	yes, err := m.IsProof(root)
	if err != nil {
		return nil, err
	}
	if !yes {
		return nil, ast.ErrASTDomain
	}
	it := &ProofPostOrder{manager: m, todo: []ast.ASTHandle{root}, visited: make(map[ast.ASTHandle]struct{})}
	it.self = it
	return it, nil
}

func (it *ProofPostOrder) open() error {
	if it == nil || it.self != it || it.manager == nil {
		return ast.ErrASTHandle
	}
	_, err := it.manager.ProofsEnabled()
	return err
}

func (it *ProofPostOrder) HasNext() (bool, error) {
	if err := it.open(); err != nil {
		return false, err
	}
	return len(it.todo) != 0, nil
}

// Next returns a null handle after the iterator is exhausted, matching the
// source iterator's nullptr result.
func (it *ProofPostOrder) Next() (ast.ASTHandle, error) {
	if err := it.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	for len(it.todo) != 0 {
		last := len(it.todo) - 1
		current := it.todo[last]
		if _, done := it.visited[current]; done {
			it.todo = it.todo[:last]
			continue
		}
		n, err := it.manager.GetProofNumParents(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		unvisited := false
		for i := uint32(0); i < n; i++ {
			parent, err := it.manager.GetProofParent(current, i)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			yes, err := it.manager.IsProof(parent)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if !yes {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
			if _, done := it.visited[parent]; !done {
				it.todo = append(it.todo, parent)
				unvisited = true
			}
		}
		if unvisited {
			continue
		}
		it.visited[current] = struct{}{}
		it.todo = it.todo[:last]
		return current, nil
	}
	return ast.ASTHandle{}, nil
}
