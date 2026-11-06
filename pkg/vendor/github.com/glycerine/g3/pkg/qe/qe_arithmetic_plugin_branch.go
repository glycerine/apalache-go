package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticBranchKind uint8

const (
	ArithmeticInfinityBranch ArithmeticBranchKind = iota
	ArithmeticEqualityBranch
	ArithmeticOpenBranch
	ArithmeticStrictBranch
)

type ArithmeticBranchSelection struct {
	Kind  ArithmeticBranchKind
	Lower bool
	Index uint32
	Bound *ArithmeticBound
}

func (p *ArithmeticPlugin) SelectBranch(variable *ast.ContainsApp, formula ast.ASTHandle, value *big.Rat) (ArithmeticBranchSelection, error) {
	if value == nil || !value.IsInt() || value.Sign() < 0 || !value.Num().IsUint64() {
		return ArithmeticBranchSelection{}, ast.ErrASTDomain
	}
	e, target, err := p.entry(variable, formula)
	if err != nil {
		return ArithmeticBranchSelection{}, err
	}
	policy, err := p.branchPolicy(e, target)
	if err != nil {
		return ArithmeticBranchSelection{}, err
	}
	v := value.Num().Uint64()
	if v == 0 {
		return ArithmeticBranchSelection{Kind: ArithmeticInfinityBranch, Lower: policy.Lower}, nil
	}
	v--
	direction := ArithmeticUpperBound
	if policy.Lower {
		direction = ArithmeticLowerBound
	}
	non := e.processor.Bounds(false, direction)
	if policy.Real {
		if v < uint64(2*len(non)) {
			idx := uint32(v / 2)
			kind := ArithmeticEqualityBranch
			if v%2 == 1 {
				kind = ArithmeticOpenBranch
			}
			return ArithmeticBranchSelection{kind, policy.Lower, idx, non[idx]}, nil
		}
		v -= uint64(2 * len(non))
	} else if v < uint64(len(non)) {
		return ArithmeticBranchSelection{ArithmeticEqualityBranch, policy.Lower, uint32(v), non[v]}, nil
	} else {
		v -= uint64(len(non))
	}
	strict := e.processor.Bounds(true, direction)
	if v >= uint64(len(strict)) {
		return ArithmeticBranchSelection{}, errors.Join(ast.ErrASTDomain)
	}
	return ArithmeticBranchSelection{ArithmeticStrictBranch, policy.Lower, uint32(v), strict[v]}, nil
}
