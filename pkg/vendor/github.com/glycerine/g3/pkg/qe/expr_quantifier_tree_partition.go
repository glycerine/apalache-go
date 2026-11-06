package qe

// Source: src/qe/qe.cpp, pop formula partition and partition_vars dispatch.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// exprQETreePartition owns the closed, mixed, and open formulas. Groups hold
// indexes into the current node's variable list in source partition order.
type exprQETreePartition struct {
	manager             *ast.ASTManager
	closed, mixed, open ast.ASTRef
	groups              [][]uint32
	split               bool
}

func partitionExprQETree(node *exprQETree, variables *exprQEBranchVariables, partitioner *ConjunctionPartitioner) (result *exprQETreePartition, err error) {
	if node == nil || node.manager == nil || node.formula == (ast.ASTRef{}) || variables == nil || variables.manager != node.manager || partitioner == nil || partitioner.manager != node.manager {
		return nil, ast.ErrASTHandle
	}
	m := node.manager
	result = &exprQETreePartition{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	handles := make([]ast.ASTHandle, len(node.variables))
	contains := make([]*ast.ContainsApp, len(node.variables))
	for i, ref := range node.variables {
		handles[i], err = m.RefGet(ref)
		if err != nil {
			return result, err
		}
		entry, ok := variables.index[handles[i]]
		if !ok {
			return result, ast.ErrASTDomain
		}
		contains[i] = variables.entries[entry].contains
	}
	formula, err := m.RefGet(node.formula)
	if err != nil {
		return result, err
	}
	result.closed, result.mixed, result.open, err = partitioner.GetPartition(formula, handles)
	if err != nil {
		return result, err
	}
	open, err := m.RefGet(result.open)
	if err != nil {
		return result, err
	}
	kind, err := m.GetKind(open)
	if err != nil {
		return result, err
	}
	if kind != ast.ASTApp {
		return result, nil
	}
	family, err := m.GetFamilyID(open)
	if err != nil {
		return result, err
	}
	op, err := m.GetDeclKind(open)
	if err != nil {
		return result, err
	}
	if family != m.BasicFamilyID() || op != ast.ASTOpAnd {
		return result, nil
	}
	count, err := m.GetNumArgs(open)
	if err != nil {
		return result, err
	}
	conjuncts := make([]ast.ASTHandle, count)
	for i := range conjuncts {
		conjuncts[i], err = m.GetArg(open, uint32(i))
		if err != nil {
			return result, err
		}
	}
	result.groups, result.split, err = partitioner.PartitionVariables(contains, conjuncts)
	return result, err
}

func (p *exprQETreePartition) Close() error {
	if p == nil || p.manager == nil {
		return nil
	}
	if err := closePartitionRefs(p.manager, &p.closed, &p.mixed, &p.open); err != nil {
		return err
	}
	p.manager = nil
	return nil
}

// popVariables returns borrowed variable handles from the last group. The
// tree must stay alive and its variable list must not change during extraction.
func (p *exprQETreePartition) popVariables(node *exprQETree) ([]ast.ASTHandle, bool, error) {
	if p == nil || p.manager == nil || node == nil || node.manager != p.manager {
		return nil, false, ast.ErrASTHandle
	}
	if len(p.groups) == 0 {
		return nil, false, nil
	}
	group := p.groups[len(p.groups)-1]
	result := make([]ast.ASTHandle, len(group))
	for i, index := range group {
		if index >= uint32(len(node.variables)) {
			return nil, false, ast.ErrASTDomain
		}
		var err error
		result[i], err = p.manager.RefGet(node.variables[index])
		if err != nil {
			return nil, false, err
		}
	}
	p.groups = p.groups[:len(p.groups)-1]
	return result, true, nil
}
