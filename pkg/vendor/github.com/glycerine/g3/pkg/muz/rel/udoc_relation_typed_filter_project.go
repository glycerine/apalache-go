package rel

// Source: udoc_plugin::filter_proj_fn and mk_filter_interpreted_and_project_fn
// in src/muz/rel/udoc_relation.cpp.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// FilterInterpretedAndProject keeps the source guard equality classes while
// discarding columns. The source relation remains unchanged.
func (r *TypedUDocRelation) FilterInterpretedAndProject(condition ast.ASTHandle, removed []uint32) (*TypedUDocRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	used, err := ast.NewASTUsedVars(r.factory.manager)
	if err != nil {
		return nil, err
	}
	if err = used.Collect(condition); err != nil {
		return nil, err
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sort := used.Contains(variable)
		if !sort.IsNull() && (uint64(variable) >= uint64(len(r.sorts)) || r.sorts[variable] != sort) {
			return nil, ast.ErrASTDomain
		}
	}
	seen := make([]bool, len(r.sorts))
	for index, column := range removed {
		if column >= uint32(len(r.sorts)) || seen[column] || index != 0 && removed[index-1] >= column {
			return nil, arena.ErrBounds
		}
		seen[column] = true
	}
	sorts := make([]ast.ASTHandle, 0, len(r.sorts)-len(removed))
	widths := make([]uint32, 0, cap(sorts))
	for column, sort := range r.sorts {
		if !seen[column] {
			sorts = append(sorts, sort)
			widths = append(widths, r.relation.widths[column])
		}
	}
	destination, err := r.factory.managerFor(widths)
	if err != nil {
		return nil, err
	}
	filtered, err := r.relation.FilterGuardAndProject(r.factory.manager, condition, removed, destination)
	if err != nil {
		return nil, err
	}
	result := &TypedUDocRelation{factory: r.factory, sorts: sorts, relation: filtered}
	expected, err := r.checkerFilterProject(condition, removed)
	return r.checkedResult(result, expected, err)
}

// udocProjectEqualityRange implements source is_var_range for equality
// extraction. Finite variables participate in fused projection equalities;
// the ordinary guard compiler keeps its narrower BV/Boolean language.
func udocProjectEqualityRange(m *ast.ASTManager, expression ast.ASTHandle) (variable, low, high uint32, matched bool, err error) {
	variable, low, high, matched, err = udocGuardRange(m, expression)
	if err != nil || matched {
		return
	}
	kind, err := m.GetKind(expression)
	if err != nil || kind != ast.ASTVar {
		return 0, 0, 0, false, err
	}
	sort, err := m.GetSort(expression)
	if err != nil {
		return 0, 0, 0, false, err
	}
	size, finite, err := m.DatalogFiniteSortSize(sort)
	if err != nil || !finite {
		return 0, 0, 0, false, err
	}
	var width uint32
	for size != 0 {
		width++
		size /= 2
	}
	if width == 0 {
		return 0, 0, 0, false, ErrDOC
	}
	variable, err = m.GetVarIndex(expression)
	return variable, 0, width - 1, err == nil, err
}
