package rel

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func (r *TypedUDocRelation) sameShape(other *TypedUDocRelation) bool {
	if r == nil || other == nil || r.open() != nil || other.open() != nil || r.factory != other.factory || len(r.sorts) != len(other.sorts) {
		return false
	}
	for i := range r.sorts {
		if r.sorts[i] != other.sorts[i] {
			return false
		}
	}
	return true
}

func (r *TypedUDocRelation) UnionFrom(source, delta *TypedUDocRelation) (resultErr error) {
	if !r.sameShape(source) || delta != nil && !r.sameShape(delta) {
		return ErrDOC
	}
	var old, sourceFormula, deltaOld ast.ASTRef
	if r.checker != nil {
		m := r.factory.manager
		owner := &typedUDocFormulaOwner{manager: m}
		defer func() {
			resultErr = finishTypedUDocFormula(owner, resultErr)
		}()
		var err error
		old, err = r.checkerProject(nil)
		if old != (ast.ASTRef{}) {
			owner.roots = append(owner.roots, old)
		}
		if err != nil {
			return err
		}
		if source.checker == nil || delta != nil && delta.checker == nil {
			return ErrRelationVerification
		}
		sourceFormula, err = source.checkerProject(nil)
		if sourceFormula != (ast.ASTRef{}) {
			owner.roots = append(owner.roots, sourceFormula)
		}
		if err != nil {
			return err
		}
		if delta != nil {
			deltaOld, err = delta.checkerProject(nil)
			if deltaOld != (ast.ASTRef{}) {
				owner.roots = append(owner.roots, deltaOld)
			}
			if err != nil {
				return err
			}
		}
	}
	var deltaCore *UDocRelation
	if delta != nil {
		deltaCore = delta.relation
	}
	if err := r.relation.UnionFrom(source.relation, deltaCore); err != nil {
		return err
	}
	if r.checker != nil {
		return r.checkerUnion(sourceFormula, delta, old, deltaOld)
	}
	return nil
}

func (r *TypedUDocRelation) FilterEqual(column uint32, value ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	if column >= uint32(len(r.sorts)) {
		return arena.ErrBounds
	}
	sort, err := r.factory.manager.GetSort(value)
	if err != nil || sort != r.sorts[column] {
		return ast.ErrASTDomain
	}
	if number, width, matched, err := r.factory.manager.BitVectorNumeralBigInt(value); err != nil {
		return err
	} else if matched {
		if width != r.relation.widths[column] {
			return ast.ErrASTDomain
		}
		if err = r.relation.FilterEqualBig(column, number); err != nil {
			return err
		}
		return r.checkerEqual(column, value)
	}
	number, matched, err := r.factory.manager.DatalogNumeralValue(value)
	if err != nil || !matched {
		return ast.ErrASTDomain
	}
	if err = r.relation.FilterEqual(column, number); err != nil {
		return err
	}
	return r.checkerEqual(column, value)
}

func (r *TypedUDocRelation) FilterIdentical(columns []uint32) error {
	if err := r.open(); err != nil {
		return err
	}
	if len(columns) == 0 {
		return ErrDOC
	}
	for _, column := range columns {
		if column >= uint32(len(r.sorts)) || r.sorts[column] != r.sorts[columns[0]] {
			return ast.ErrASTDomain
		}
	}
	if err := r.relation.FilterIdentical(columns); err != nil {
		return err
	}
	return r.checkerIdentical(columns)
}

func (r *TypedUDocRelation) FilterInterpreted(condition ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.relation.FilterGuard(r.factory.manager, condition); err != nil {
		return err
	}
	return r.checkerFilter(condition)
}

func (r *TypedUDocRelation) Rename(cycle []uint32) (*TypedUDocRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if len(cycle) == 0 {
		return r.Clone()
	}
	sorts := append([]ast.ASTHandle(nil), r.sorts...)
	seen := make([]bool, len(sorts))
	for _, column := range cycle {
		if column >= uint32(len(sorts)) || seen[column] {
			return nil, arena.ErrBounds
		}
		seen[column] = true
	}
	for i, destination := range cycle {
		source := cycle[(i+1)%len(cycle)]
		sorts[destination] = r.sorts[source]
	}
	renamed, err := r.relation.Rename(cycle)
	if err != nil {
		return nil, err
	}
	result := &TypedUDocRelation{factory: r.factory, sorts: sorts, relation: renamed}
	var expected ast.ASTRef
	if r.checker != nil {
		expected, err = r.checker.renameFormulaRef(r.checker.formula, r.sorts, sorts, cycle)
	}
	return r.checkedResult(result, expected, err)
}
