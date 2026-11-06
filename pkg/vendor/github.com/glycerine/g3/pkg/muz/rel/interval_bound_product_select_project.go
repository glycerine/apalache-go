package rel

// Source: relation_manager::default_relation_select_equal_and_project_fn
// clones its input, applies filter_equal, then removes the selected column.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *IntervalBoundProduct) SelectEqualAndProject(column uint32, value *big.Rat) (*IntervalBoundProduct, error) {
	if p == nil || p.interval == nil || p.bound == nil || value == nil {
		return nil, ast.ErrASTDomain
	}
	arity, err := p.interval.Arity()
	if err != nil || column >= arity || int(arity) != len(p.bound.columns) {
		return nil, ast.ErrASTDomain
	}
	working, err := p.Clone()
	if err != nil {
		return nil, err
	}
	if err = working.FilterEqual(column, value); err != nil {
		return nil, err
	}
	return working.Project([]uint32{column})
}
