package rel

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// SelectEqualAndProject implements relation_manager's default relation
// transformer for the interval-vector plugin: clone, filter the selected
// column to the numeral, then project that column away.
func (r *IntervalRelation) SelectEqualAndProject(value *big.Rat, column uint32) (*IntervalRelation, error) {
	if r == nil || r.values == nil || value == nil || uint64(column) >= uint64(len(r.values.columns)) {
		return nil, ast.ErrASTDomain
	}
	auxiliary, err := r.Clone()
	if err != nil {
		return nil, err
	}
	if err = auxiliary.FilterEqual(column, value); err != nil {
		return nil, err
	}
	return auxiliary.Project([]uint32{column})
}
