package qe

// Source: src/qe/qe.cpp, update_current with apply=false.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type exprQETreeStatus uint8

const (
	exprQEChooseVariable exprQETreeStatus = iota
	exprQENeedPropagation
)

func seekExprQETree(root *exprQETree, variables *exprQEBranchVariables, evaluator *model.ModelEvaluator) (current *exprQETree, status exprQETreeStatus, branch big.Rat, err error) {
	if root == nil || root.manager == nil || root.parent != nil || variables == nil || variables.manager != root.manager || evaluator == nil {
		return nil, status, branch, ast.ErrASTHandle
	}
	m := root.manager
	current = root
	for {
		if current.isUnit() {
			current = current.children[0]
			continue
		}
		if current.selected == (ast.ASTRef{}) {
			return current, exprQEChooseVariable, branch, nil
		}
		variable, e := m.RefGet(current.selected)
		if e != nil {
			return nil, status, branch, e
		}
		proxy, e := variables.branch(variable)
		if e != nil {
			return nil, status, branch, e
		}
		branch, found, e := exprQEEvalBranch(m, evaluator, proxy)
		if e != nil {
			return nil, status, branch, e
		}
		if !found || branch.Cmp(&current.branches) >= 0 {
			branch.SetInt64(0)
		}
		index, ok := current.branchIndex[branch.RatString()]
		if !ok {
			return current, exprQENeedPropagation, branch, nil
		}
		current = current.children[index]
		if current.formula == (ast.ASTRef{}) {
			return current, exprQEChooseVariable, branch, nil
		}
	}
}
