package rel

import "github.com/glycerine/g3/pkg/ast"

func productTableMask(sorts []ast.ASTHandle, mask []bool) []bool {
	if mask != nil {
		return append([]bool(nil), mask...)
	}
	full := make([]bool, len(sorts))
	for i := range full {
		full[i] = true
	}
	return full
}

func productTableInnerSorts(sorts []ast.ASTHandle, mask []bool) []ast.ASTHandle {
	var selected []ast.ASTHandle
	for i, sort := range sorts {
		if mask[i] {
			selected = append(selected, sort)
		}
	}
	return selected
}

func productTableColumnMap(mask []bool) []int {
	columns := make([]int, len(mask))
	for i := range columns {
		columns[i] = -1
		if mask[i] {
			columns[i] = 0
			for j := 0; j < i; j++ {
				if mask[j] {
					columns[i]++
				}
			}
		}
	}
	return columns
}

// fullTableish builds the table part of sieve_relation_plugin::full. The
// ignored columns keep their outer sorts in the mask; table storage contains
// only sorts accepted by the finite table manager.
func (a *udocTableProductAttempt) fullTableish(sorts []ast.ASTHandle, like FiniteTable) (FiniteTable, []bool, error) {
	mask := make([]bool, len(sorts))
	var innerSorts []ast.ASTHandle
	for i, sort := range sorts {
		size, err := a.manager.manager.GetSortSize(sort)
		if err != nil {
			return nil, nil, err
		}
		finite := size.IsFinite()
		if !finite {
			continue
		}
		domain, err := size.Size()
		if err != nil {
			return nil, nil, err
		}
		if domain == 0 {
			return nil, nil, ErrUDocTableProduct
		}
		mask[i] = true
		innerSorts = append(innerSorts, sort)
	}
	signature, err := tableRelationSignature(a.manager, innerSorts)
	if err != nil {
		return nil, nil, err
	}
	table, err := a.fullUnaryFinite(signature, like)
	return table, mask, err
}

// joinProductTableComponents mirrors sieve_relation_plugin::join_fn: an
// equality contributes to the inner join only when both endpoint columns are
// represented by their respective table components.
func joinProductTableComponents(manager *RelationManager, leftSorts []ast.ASTHandle, left FiniteTable, leftMask []bool, rightSorts []ast.ASTHandle, right FiniteTable, rightMask []bool, leftColumns, rightColumns []uint32) (FiniteTable, []bool, error) {
	if manager == nil || left == nil || right == nil || len(leftColumns) != len(rightColumns) || len(leftMask) != len(leftSorts) || len(rightMask) != len(rightSorts) {
		return nil, nil, ErrUDocTableProduct
	}
	leftInnerSorts := productTableInnerSorts(leftSorts, leftMask)
	rightInnerSorts := productTableInnerSorts(rightSorts, rightMask)
	leftSignature, err := tableRelationSignature(manager, leftInnerSorts)
	if err != nil {
		return nil, nil, err
	}
	rightSignature, err := tableRelationSignature(manager, rightInnerSorts)
	if err != nil {
		return nil, nil, err
	}
	actual, err := left.Signature()
	if err != nil || !leftSignature.equal(actual) {
		return nil, nil, ErrUDocTableProduct
	}
	actual, err = right.Signature()
	if err != nil || !rightSignature.equal(actual) {
		return nil, nil, ErrUDocTableProduct
	}
	leftMap, rightMap := productTableColumnMap(leftMask), productTableColumnMap(rightMask)
	var innerLeft, innerRight []int
	for i, leftColumn := range leftColumns {
		rightColumn := rightColumns[i]
		if leftColumn >= uint32(len(leftSorts)) || rightColumn >= uint32(len(rightSorts)) || leftSorts[leftColumn] != rightSorts[rightColumn] {
			return nil, nil, ErrUDocTableProduct
		}
		if leftMap[leftColumn] >= 0 && rightMap[rightColumn] >= 0 {
			innerLeft = append(innerLeft, leftMap[leftColumn])
			innerRight = append(innerRight, rightMap[rightColumn])
		}
	}
	joined, err := finiteTableJoin(left, right, innerLeft, innerRight)
	if err != nil {
		return nil, nil, err
	}
	resultMask := append(append([]bool(nil), leftMask...), rightMask...)
	return joined, resultMask, nil
}
