package rel

// Source: dl_sieve_relation.cpp project and join dispatch. Join equalities
// touching an ignored column are deliberately dropped by the source sieve.

import "github.com/glycerine/g3/pkg/ast"

func (r *UDocSieve) Project(removed []uint32) (*UDocSieve, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	var innerRemoved []uint32
	for i, column := range removed {
		if column >= uint32(len(r.sorts)) || i > 0 && removed[i-1] >= column {
			return nil, ErrSieve
		}
		if index := r.sigToInner[column]; index >= 0 {
			innerRemoved = append(innerRemoved, uint32(index))
		}
	}
	sorts := make([]ast.ASTHandle, 0, len(r.sorts)-len(removed))
	mask := make([]bool, 0, cap(sorts))
	next := 0
	for column, sort := range r.sorts {
		if next < len(removed) && removed[next] == uint32(column) {
			next++
			continue
		}
		sorts = append(sorts, sort)
		mask = append(mask, r.innerColumns[column])
	}
	var inner *TypedUDocRelation
	var err error
	if len(innerRemoved) == 0 {
		inner, err = r.inner.Clone()
	} else {
		inner, err = r.inner.Project(innerRemoved)
	}
	return finishUDocSieve(sorts, mask, inner, err)
}

func (r *UDocSieve) Join(other *UDocSieve, left, right []uint32) (*UDocSieve, error) {
	if r.open() != nil || other.open() != nil {
		return nil, ErrSieve
	}
	return joinUDocSieve(r.inner, r.sorts, r.innerColumns, r.sigToInner, other.inner, other.sorts, other.innerColumns, other.sigToInner, left, right)
}

func (r *UDocSieve) JoinUDoc(other *TypedUDocRelation, left, right []uint32) (*UDocSieve, error) {
	if r.open() != nil || other == nil || other.open() != nil || other.FormulaChecked() {
		return nil, ErrSieve
	}
	mask, columns := rawUDocSieveColumns(len(other.sorts))
	return joinUDocSieve(r.inner, r.sorts, r.innerColumns, r.sigToInner, other, other.sorts, mask, columns, left, right)
}

func (r *TypedUDocRelation) JoinSieve(other *UDocSieve, left, right []uint32) (*UDocSieve, error) {
	if r == nil || r.open() != nil || r.FormulaChecked() || other.open() != nil {
		return nil, ErrSieve
	}
	mask, columns := rawUDocSieveColumns(len(r.sorts))
	return joinUDocSieve(r, r.sorts, mask, columns, other.inner, other.sorts, other.innerColumns, other.sigToInner, left, right)
}

func rawUDocSieveColumns(size int) ([]bool, []int) {
	mask, columns := make([]bool, size), make([]int, size)
	for i := range mask {
		mask[i], columns[i] = true, i
	}
	return mask, columns
}

func joinUDocSieve(leftInner *TypedUDocRelation, leftSorts []ast.ASTHandle, leftMask []bool, leftMap []int, rightInner *TypedUDocRelation, rightSorts []ast.ASTHandle, rightMask []bool, rightMap []int, left, right []uint32) (*UDocSieve, error) {
	if leftInner.factory != rightInner.factory || len(left) != len(right) {
		return nil, ErrSieve
	}
	var innerLeft, innerRight []uint32
	for i, column := range left {
		other := right[i]
		if column >= uint32(len(leftSorts)) || other >= uint32(len(rightSorts)) || leftSorts[column] != rightSorts[other] {
			return nil, ast.ErrASTDomain
		}
		if leftMap[column] < 0 || rightMap[other] < 0 {
			continue
		}
		innerLeft = append(innerLeft, uint32(leftMap[column]))
		innerRight = append(innerRight, uint32(rightMap[other]))
	}
	sorts := append(append([]ast.ASTHandle(nil), leftSorts...), rightSorts...)
	mask := append(append([]bool(nil), leftMask...), rightMask...)
	inner, err := leftInner.Join(rightInner, innerLeft, innerRight)
	return finishUDocSieve(sorts, mask, inner, err)
}
