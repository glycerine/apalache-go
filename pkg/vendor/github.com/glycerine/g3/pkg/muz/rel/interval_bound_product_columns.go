package rel

// Source: dl_compiler.cpp make_add_unbound_column joins the working relation
// with a full unary relation of the selected kind. Repeated head variables are
// assembled by adding such a column and applying the product's componentwise
// identical-column filter.

import "github.com/glycerine/g3/pkg/ast"

func (p *IntervalBoundProduct) validateColumnSignature(manager *ast.ASTManager, sorts []ast.ASTHandle) error {
	if p == nil || p.interval == nil || p.bound == nil || manager == nil {
		return ast.ErrASTDomain
	}
	arity, err := p.interval.Arity()
	if err != nil || int(arity) != len(sorts) || len(p.bound.columns) != len(sorts) {
		return ast.ErrASTDomain
	}
	selected, err := CanSelectIntervalBoundProduct(manager, sorts, p.kinds)
	if err != nil {
		return err
	}
	if !selected {
		return ast.ErrASTDomain
	}
	return nil
}

// AppendFullColumn returns an independent product with one unconstrained
// arithmetic column appended. The receiver and its requested component order
// are unchanged.
func (p *IntervalBoundProduct) AppendFullColumn(manager *ast.ASTManager, sorts []ast.ASTHandle, sort ast.ASTHandle) (*IntervalBoundProduct, error) {
	if err := p.validateColumnSignature(manager, sorts); err != nil {
		return nil, err
	}
	full, selected, err := NewSelectedIntervalBoundProduct(manager, []ast.ASTHandle{sort}, p.kinds, false)
	if err != nil {
		return nil, err
	}
	if !selected {
		return nil, ast.ErrASTDomain
	}
	return JoinIntervalBoundProducts(p, full, nil, nil)
}

// DuplicateColumn appends a full column and constrains it to be identical to
// an existing column in both product components.
func (p *IntervalBoundProduct) DuplicateColumn(manager *ast.ASTManager, sorts []ast.ASTHandle, column uint32) (*IntervalBoundProduct, error) {
	if uint64(column) >= uint64(len(sorts)) {
		return nil, ast.ErrASTDomain
	}
	result, err := p.AppendFullColumn(manager, sorts, sorts[column])
	if err != nil {
		return nil, err
	}
	if err = result.FilterIdentical(column, uint32(len(sorts))); err != nil {
		return nil, err
	}
	return result, nil
}
