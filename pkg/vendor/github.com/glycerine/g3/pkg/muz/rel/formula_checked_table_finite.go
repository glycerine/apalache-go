package rel

// Source: src/muz/rel/check_relation.cpp relation_base operations used by the
// live finite-table execution boundary.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Signature exposes the concrete checked relation signature.
func (r *FormulaCheckedTable) Signature() (Signature, error) {
	if err := r.open(); err != nil {
		return Signature{}, err
	}
	return r.table.Signature()
}

// Add checks an encoded finite-table row against the relation formula.
func (r *FormulaCheckedTable) Add(fact Fact) (bool, error) {
	return r.AddTableFact(fact)
}

// Contains decodes an encoded row and verifies the membership result against
// the relation formula; it deliberately does not bypass the checker table.
func (r *FormulaCheckedTable) Contains(fact Fact) (found bool, resultErr error) {
	if err := r.open(); err != nil {
		return false, err
	}
	values, err := r.table.manager.DecodeFact(r.table.sorts, fact)
	if err != nil {
		return false, err
	}
	defer func() {
		for i := len(values) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&values[i]))
		}
	}()
	handles := make([]ast.ASTHandle, len(values))
	for i := range values {
		handles[i], err = r.manager.RefGet(values[i])
		if err != nil {
			return false, err
		}
	}
	return r.ContainsFact(handles)
}

// Len returns the implementation-under-check's exact finite row count.
func (r *FormulaCheckedTable) Len() (int, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.table.Table().Len()
}

// CloneFinite preserves the checked representation across typed finite-table
// registers and manager storage.
func (r *FormulaCheckedTable) CloneFinite() (FiniteTable, error) {
	clone, err := r.Clone()
	if err != nil {
		return nil, err
	}
	return clone, nil
}

var _ FiniteTable = (*FormulaCheckedTable)(nil)
