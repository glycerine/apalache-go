package rel

// Source: interval_relation::add_fact and bound_relation::add_fact inside an
// aligned product_relation. Validation completes before either component is
// mutated, and the bound update cannot fail after the shared arity check.

import "github.com/glycerine/g3/pkg/ast"

func (p *IntervalBoundProduct) AddFactExpressions(manager *ast.ASTManager, sorts, values []ast.ASTHandle) error {
	if p == nil || p.interval == nil || p.bound == nil || manager == nil || !intervalBoundProductKinds(p.kinds) || len(sorts) != len(values) || len(values) != len(p.bound.columns) || !CanHandleIntervalSorts(manager, sorts) {
		return ast.ErrASTDomain
	}
	for index, value := range values {
		actual, err := manager.GetSort(value)
		if err != nil || actual != sorts[index] {
			return ast.ErrASTDomain
		}
		_, _, numeral, err := manager.ArithmeticNumeralText(value)
		if err != nil {
			return err
		}
		if !numeral {
			return ast.ErrASTDomain
		}
	}
	if err := p.interval.AddFactExpressions(manager, sorts, values); err != nil {
		return err
	}
	return p.bound.AddFact(values)
}
