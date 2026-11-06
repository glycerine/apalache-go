package rel

// Source: src/muz/rel/dl_sieve_relation.cpp join_fn and transformer_fn.

import "github.com/glycerine/g3/pkg/ast"

func (r *IntervalSieve) Project(removed []uint32) (*IntervalSieve, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	drop := make([]bool, len(r.sorts))
	innerRemoved := make([]uint32, 0, len(removed))
	for _, column := range removed {
		if uint64(column) >= uint64(len(drop)) || drop[column] {
			return nil, ast.ErrASTDomain
		}
		drop[column] = true
		if inner := r.sigToInner[column]; inner >= 0 {
			innerRemoved = append(innerRemoved, uint32(inner))
		}
	}
	sorts := make([]ast.ASTHandle, 0, len(r.sorts)-len(removed))
	mask := make([]bool, 0, len(r.sorts)-len(removed))
	for column := range r.sorts {
		if !drop[column] {
			sorts = append(sorts, r.sorts[column])
			mask = append(mask, r.innerCols[column])
		}
	}
	inner := r.inner
	var err error
	if len(innerRemoved) == 0 {
		inner, err = inner.Clone()
	} else {
		inner, err = inner.Project(innerRemoved)
	}
	if err != nil {
		return nil, err
	}
	return newIntervalSieve(r.manager, sorts, mask, inner)
}

func JoinIntervalSieves(left, right *IntervalSieve, leftColumns, rightColumns []uint32) (*IntervalSieve, error) {
	if left == nil || right == nil || left.open() != nil || right.open() != nil || left.manager != right.manager || len(leftColumns) != len(rightColumns) {
		return nil, ast.ErrASTDomain
	}
	innerLeft := make([]uint32, 0, len(leftColumns))
	innerRight := make([]uint32, 0, len(rightColumns))
	for i := range leftColumns {
		lc, rc := leftColumns[i], rightColumns[i]
		if uint64(lc) >= uint64(len(left.sorts)) || uint64(rc) >= uint64(len(right.sorts)) {
			return nil, ast.ErrASTDomain
		}
		li, ri := left.sigToInner[lc], right.sigToInner[rc]
		// Source ignores an equality unless both ends belong to the inner relations.
		if li >= 0 && ri >= 0 {
			innerLeft = append(innerLeft, uint32(li))
			innerRight = append(innerRight, uint32(ri))
		}
	}
	inner, err := JoinIntervalRelations(left.inner, right.inner, innerLeft, innerRight)
	if err != nil {
		return nil, err
	}
	sorts := append(append([]ast.ASTHandle(nil), left.sorts...), right.sorts...)
	mask := append(append([]bool(nil), left.innerCols...), right.innerCols...)
	return newIntervalSieve(left.manager, sorts, mask, inner)
}

// JoinIntervalSieveWithInterval is the source sieve/non-sieve join used when a
// product component is already an ordinary interval relation.
func JoinIntervalSieveWithInterval(left *IntervalSieve, right *IntervalRelation, rightSorts []ast.ASTHandle, leftColumns, rightColumns []uint32) (*IntervalSieve, error) {
	if left == nil || right == nil || left.open() != nil || !CanHandleIntervalSorts(left.manager, rightSorts) || len(leftColumns) != len(rightColumns) {
		return nil, ast.ErrASTDomain
	}
	if arity, err := right.Arity(); err != nil || int(arity) != len(rightSorts) {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	innerLeft := make([]uint32, 0, len(leftColumns))
	innerRight := make([]uint32, 0, len(rightColumns))
	for i := range leftColumns {
		lc, rc := leftColumns[i], rightColumns[i]
		if uint64(lc) >= uint64(len(left.sorts)) || uint64(rc) >= uint64(len(rightSorts)) {
			return nil, ast.ErrASTDomain
		}
		if mapped := left.sigToInner[lc]; mapped >= 0 {
			innerLeft = append(innerLeft, uint32(mapped))
			innerRight = append(innerRight, rc)
		}
	}
	inner, err := JoinIntervalRelations(left.inner, right, innerLeft, innerRight)
	if err != nil {
		return nil, err
	}
	sorts := append(append([]ast.ASTHandle(nil), left.sorts...), rightSorts...)
	mask := append(append([]bool(nil), left.innerCols...), make([]bool, len(rightSorts))...)
	for i := len(left.innerCols); i < len(mask); i++ {
		mask[i] = true
	}
	return newIntervalSieve(left.manager, sorts, mask, inner)
}

// JoinIntervalWithIntervalSieve is the symmetric ordinary/sieve source path.
func JoinIntervalWithIntervalSieve(left *IntervalRelation, leftSorts []ast.ASTHandle, right *IntervalSieve, leftColumns, rightColumns []uint32) (*IntervalSieve, error) {
	if right == nil || left == nil || right.open() != nil || !CanHandleIntervalSorts(right.manager, leftSorts) || len(leftColumns) != len(rightColumns) {
		return nil, ast.ErrASTDomain
	}
	if arity, err := left.Arity(); err != nil || int(arity) != len(leftSorts) {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	innerLeft := make([]uint32, 0, len(leftColumns))
	innerRight := make([]uint32, 0, len(rightColumns))
	for i := range leftColumns {
		lc, rc := leftColumns[i], rightColumns[i]
		if uint64(lc) >= uint64(len(leftSorts)) || uint64(rc) >= uint64(len(right.sorts)) {
			return nil, ast.ErrASTDomain
		}
		if mapped := right.sigToInner[rc]; mapped >= 0 {
			innerLeft = append(innerLeft, lc)
			innerRight = append(innerRight, uint32(mapped))
		}
	}
	inner, err := JoinIntervalRelations(left, right.inner, innerLeft, innerRight)
	if err != nil {
		return nil, err
	}
	sorts := append(append([]ast.ASTHandle(nil), leftSorts...), right.sorts...)
	mask := make([]bool, len(leftSorts), len(sorts))
	for i := range mask {
		mask[i] = true
	}
	mask = append(mask, right.innerCols...)
	return newIntervalSieve(right.manager, sorts, mask, inner)
}
