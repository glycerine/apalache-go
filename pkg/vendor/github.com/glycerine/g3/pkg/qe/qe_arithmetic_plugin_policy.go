package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type ArithmeticBranchPolicy struct {
	Lower                       bool
	StrictCount, NonStrictCount uint32
	Real                        bool
}

func (p *ArithmeticPlugin) branchPolicy(entry *arithmeticPluginBounds, target ast.ASTHandle) (ArithmeticBranchPolicy, error) {
	lowerStrict := len(entry.processor.Bounds(true, ArithmeticLowerBound))
	upperStrict := len(entry.processor.Bounds(true, ArithmeticUpperBound))
	lowerNon := len(entry.processor.Bounds(false, ArithmeticLowerBound))
	upperNon := len(entry.processor.Bounds(false, ArithmeticUpperBound))
	integer, err := p.util.isInteger(target)
	if err != nil {
		return ArithmeticBranchPolicy{}, err
	}
	real := !integer
	lcmp, ucmp := lowerNon, upperNon
	if real {
		lcmp *= 2
		ucmp *= 2
	}
	result := ArithmeticBranchPolicy{Real: real}
	result.Lower = lcmp+lowerStrict < ucmp+upperStrict
	if result.Lower {
		result.StrictCount = uint32(lowerStrict)
		result.NonStrictCount = uint32(lowerNon)
	} else {
		result.StrictCount = uint32(upperStrict)
		result.NonStrictCount = uint32(upperNon)
	}
	return result, nil
}

func (p *ArithmeticPlugin) BranchPolicy(variable *ast.ContainsApp, formula ast.ASTHandle) (ArithmeticBranchPolicy, error) {
	e, target, err := p.entry(variable, formula)
	if err != nil {
		return ArithmeticBranchPolicy{}, err
	}
	return p.branchPolicy(e, target)
}

func (p *ArithmeticPlugin) GetNumBranches(variable *ast.ContainsApp, formula ast.ASTHandle, branches *numeral.OwnedRational) (bool, error) {
	if branches == nil {
		return false, ast.ErrASTHandle
	}
	e, target, err := p.entry(variable, formula)
	if err != nil {
		return false, err
	}
	policy, err := p.branchPolicy(e, target)
	if err != nil {
		return false, err
	}
	n := policy.StrictCount + policy.NonStrictCount + 1
	if policy.Real {
		n += policy.NonStrictCount
	}
	return true, p.manager.NumeralService().SetUnsigned(branches, n)
}
