package rel

// Source: dl_compiler.cpp make_add_constant_column constructs a unary
// singleton in the head predicate's requested kind and joins it to the working
// relation. For the bound/interval product, filter_equal constrains the
// interval component while bound_relation deliberately leaves constants
// unconstrained.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *IntervalBoundProduct) AppendConstantColumn(manager *ast.ASTManager, sorts []ast.ASTHandle, sort ast.ASTHandle, value ast.ASTHandle) (*IntervalBoundProduct, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	actual, err := manager.GetSort(value)
	if err != nil {
		return nil, err
	}
	if actual != sort {
		return nil, ast.ErrASTDomain
	}
	text, _, numeral, err := manager.ArithmeticNumeralText(value)
	if err != nil {
		return nil, err
	}
	if !numeral {
		return nil, ast.ErrASTDomain
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	result, err := p.AppendFullColumn(manager, sorts, sort)
	if err != nil {
		return nil, err
	}
	if err = result.FilterEqual(uint32(len(sorts)), number); err != nil {
		return nil, err
	}
	return result, nil
}
