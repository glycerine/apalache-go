package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp, join_fn.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// JoinFiniteIntervalProducts concatenates the input signatures and equates
// the requested pairs of columns.
func JoinFiniteIntervalProducts(left, right *FiniteIntervalProduct, leftColumns, rightColumns []uint32) (_ *FiniteIntervalProduct, resultErr error) {
	if left.open() != nil || right.open() != nil || left.manager != right.manager || len(leftColumns) != len(rightColumns) {
		return nil, ErrTableRelation
	}
	leftTable, rightTable := make([]int, len(left.sorts)), make([]int, len(right.sorts))
	leftInner, rightInner := make([]int, len(left.sorts)), make([]int, len(right.sorts))
	for i := range left.sorts {
		leftTable[i], leftInner[i] = -1, -1
	}
	for i := range right.sorts {
		rightTable[i], rightInner[i] = -1, -1
	}
	for local, global := range left.tableColumns {
		leftTable[global] = local
	}
	for local, global := range right.tableColumns {
		rightTable[global] = local
	}
	for local, global := range left.intervalColumns {
		leftInner[global] = local
	}
	for local, global := range right.intervalColumns {
		rightInner[global] = local
	}
	var tableLeft, tableRight []int
	var innerLeft, innerRight []uint32
	for i, l := range leftColumns {
		r := rightColumns[i]
		if uint64(l) >= uint64(len(left.sorts)) || uint64(r) >= uint64(len(right.sorts)) || left.sorts[l] != right.sorts[r] {
			return nil, ast.ErrASTDomain
		}
		if leftTable[l] >= 0 && rightTable[r] >= 0 {
			tableLeft, tableRight = append(tableLeft, leftTable[l]), append(tableRight, rightTable[r])
		} else if leftInner[l] >= 0 && rightInner[r] >= 0 {
			innerLeft, innerRight = append(innerLeft, uint32(leftInner[l])), append(innerRight, uint32(rightInner[r]))
		} else {
			return nil, ErrTableRelation
		}
	}
	sorts := append(append([]ast.ASTHandle(nil), left.sorts...), right.sorts...)
	mask := make([]bool, len(sorts))
	for _, column := range left.tableColumns {
		mask[column] = true
	}
	for _, column := range right.tableColumns {
		mask[len(left.sorts)+column] = true
	}
	result, err := NewFiniteIntervalProduct(left.manager, sorts, mask)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	leftRows, err := left.table.Facts()
	if err != nil {
		return nil, err
	}
	rightRows, err := right.table.Facts()
	if err != nil {
		return nil, err
	}
	for _, lrow := range leftRows {
		li := uint64(lrow[len(lrow)-1])
		if li >= uint64(len(left.intervals)) || left.intervals[li] == nil {
			return nil, ErrTableRelation
		}
		for _, rrow := range rightRows {
			matches := true
			for i, lc := range tableLeft {
				if lrow[lc] != rrow[tableRight[i]] {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
			ri := uint64(rrow[len(rrow)-1])
			if ri >= uint64(len(right.intervals)) || right.intervals[ri] == nil {
				return nil, ErrTableRelation
			}
			inner, err := JoinIntervalRelations(left.intervals[li], right.intervals[ri], innerLeft, innerRight)
			if err != nil {
				return nil, err
			}
			key := make(Fact, 0, len(result.tableColumns)+1)
			key = append(key, lrow[:len(lrow)-1]...)
			key = append(key, rrow[:len(rrow)-1]...)
			key = append(key, 0)
			if err = result.storeInner(key, inner); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
