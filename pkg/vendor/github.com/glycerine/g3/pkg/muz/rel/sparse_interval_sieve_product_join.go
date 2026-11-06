package rel

// Source: src/muz/rel/dl_product_relation.cpp join_fn with aligned concrete
// sparse-table and projected interval components.

import "github.com/glycerine/g3/pkg/ast"

func sparseIntervalJoinShape(left, right *SparseIntervalSieveProduct) sparseIntervalComponentShape {
	if left.intervalShape == sparseIntervalShapeSieve || right.intervalShape == sparseIntervalShapeSieve {
		return sparseIntervalShapeSieve
	}
	if left.intervalShape == sparseIntervalShapeRaw && right.intervalShape == sparseIntervalShapeRaw {
		return sparseIntervalShapeRaw
	}
	if left.intervalShape == sparseIntervalShapeAbsent && right.intervalShape == sparseIntervalShapeAbsent {
		return sparseIntervalShapeAbsent
	}
	raw, opposite := left, right
	if right.intervalShape == sparseIntervalShapeRaw {
		raw, opposite = right, left
	}
	if raw.intervalShape != sparseIntervalShapeRaw || opposite.intervalShape != sparseIntervalShapeAbsent {
		return sparseIntervalShapeSieve
	}
	for _, sort := range opposite.sorts {
		if !CanHandleIntervalSorts(left.manager.manager, []ast.ASTHandle{sort}) {
			return sparseIntervalShapeSieve
		}
	}
	return sparseIntervalShapeRaw
}

func JoinSparseIntervalSieveProducts(left, right *SparseIntervalSieveProduct, leftColumns, rightColumns []int) (*SparseIntervalSieveProduct, error) {
	if left == nil || right == nil || left.open() != nil || right.open() != nil || left.manager != right.manager || len(leftColumns) != len(rightColumns) {
		return nil, ErrSparseIntervalProduct
	}
	finiteLeft := make([]int, 0, len(leftColumns))
	finiteRight := make([]int, 0, len(rightColumns))
	intervalLeft := make([]uint32, len(leftColumns))
	intervalRight := make([]uint32, len(rightColumns))
	for i := range leftColumns {
		lc, rc := leftColumns[i], rightColumns[i]
		if lc < 0 || lc >= len(left.sorts) || rc < 0 || rc >= len(right.sorts) {
			return nil, ErrSparseIntervalProduct
		}
		intervalLeft[i], intervalRight[i] = uint32(lc), uint32(rc)
		ll, rl := -1, -1
		for local, outer := range left.finiteColumns {
			if outer == lc {
				ll = local
				break
			}
		}
		for local, outer := range right.finiteColumns {
			if outer == rc {
				rl = local
				break
			}
		}
		if ll >= 0 && rl >= 0 {
			finiteLeft, finiteRight = append(finiteLeft, ll), append(finiteRight, rl)
		}
	}
	finite, err := left.finite.Join(right.finite, finiteLeft, finiteRight)
	if err != nil {
		return nil, err
	}
	interval, err := JoinIntervalSieves(left.interval, right.interval, intervalLeft, intervalRight)
	if err != nil {
		_ = finite.Close()
		return nil, err
	}
	sorts := append(append([]ast.ASTHandle(nil), left.sorts...), right.sorts...)
	columns := append(append([]int(nil), left.finiteColumns...), make([]int, len(right.finiteColumns))...)
	for i, outer := range right.finiteColumns {
		columns[len(left.finiteColumns)+i] = len(left.sorts) + outer
	}
	finiteSorts := append(append([]ast.ASTHandle(nil), left.finiteSorts...), right.finiteSorts...)
	shape := sparseIntervalJoinShape(left, right)
	return &SparseIntervalSieveProduct{manager: left.manager, sorts: sorts, finiteColumns: columns, finiteSorts: finiteSorts, finite: finite, interval: interval, intervalShape: shape, intervalNullary: shape == sparseIntervalShapeRaw && len(sorts) == 0}, nil
}
