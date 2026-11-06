package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::constrain_assignment.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

func constrainExprQEBranch(node *exprQETree, variables *exprQEBranchVariables, solver *smtcore.SMTSolver, variable ast.ASTHandle, count *big.Rat) (err error) {
	if node == nil || node.manager == nil || variables == nil || variables.manager != node.manager || solver == nil || count == nil || !count.IsInt() || count.Sign() <= 0 {
		return ast.ErrASTDomain
	}
	m := node.manager
	branch, err := variables.branch(variable)
	if err != nil {
		return err
	}
	isBV, err := m.IsBitVector(variable)
	if err != nil {
		return err
	}
	if err = node.selectVariable(variable, count); err != nil {
		return err
	}
	if isBV {
		return nil
	}
	sort, err := m.GetSort(branch)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	if sort == boolean {
		if count.Cmp(big.NewRat(2, 1)) > 0 {
			return ast.ErrASTDomain
		}
		return nil
	}
	width, err := m.BitVectorSize(branch)
	if err != nil {
		return err
	}
	max := new(big.Rat).Sub(count, big.NewRat(1, 1))
	n, err := m.BitVectorNumeralRat(max, width)
	if err != nil {
		return err
	}
	nRef, err := m.NewRef(n)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&nRef)) }()
	bound, err := m.MkBitVectorULE(branch, n)
	if err != nil {
		return err
	}
	return assertExprQETreeConstraint(node, solver, true, bound)
}
