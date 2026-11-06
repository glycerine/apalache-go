package rel

func sparseJoinProjectFunctionalCount(left, right *SparseTable, leftColumns, rightColumns, removed []int) (int, error) {
	return sparseJoinProjectFunctionalCountSignatures(left.signature, left.functional, right.signature, right.functional, leftColumns, rightColumns, removed)
}

func sparseJoinProjectFunctionalCountSignatures(leftSignature Signature, leftFunctional int, rightSignature Signature, rightFunctional int, leftColumns, rightColumns, removed []int) (int, error) {
	leftFirst := leftSignature.Len() - leftFunctional
	rightFirst := rightSignature.Len() - rightFunctional
	domains := make([]uint64, 0, leftSignature.Len()+rightSignature.Len())
	domains = append(domains, leftSignature.domains[:leftFirst]...)
	domains = append(domains, rightSignature.domains[:rightFirst]...)
	domains = append(domains, leftSignature.domains[leftFirst:]...)
	domains = append(domains, rightSignature.domains[rightFirst:]...)
	joinedSignature, err := NewSignature(domains)
	if err != nil {
		return 0, err
	}
	if _, err = projectedSignature(joinedSignature, removed); err != nil {
		return 0, err
	}
	firstFunctional := leftFirst + rightFirst
	equivalence := newSparseColumnEquivalence(joinedSignature.Len())
	for i := range leftColumns {
		leftIndex := leftColumns[i]
		if leftIndex >= leftFirst {
			leftIndex = firstFunctional + leftIndex - leftFirst
		}
		rightIndex := rightColumns[i]
		if rightIndex < rightFirst {
			rightIndex += leftFirst
		} else {
			rightIndex = firstFunctional + leftFunctional + rightIndex - rightFirst
		}
		equivalence.merge(leftIndex, rightIndex)
	}
	remaining := make([]int, joinedSignature.Len())
	for column := 0; column < firstFunctional; column++ {
		remaining[equivalence.find(column)]++
	}
	mergesRows := false
	removedFunctional := 0
	for _, column := range removed {
		if column >= firstFunctional {
			removedFunctional++
			continue
		}
		root := equivalence.find(column)
		if remaining[root] > 1 {
			remaining[root]--
		} else {
			mergesRows = true
			break
		}
	}
	if mergesRows {
		return 0, nil
	}
	return leftFunctional + rightFunctional - removedFunctional, nil
}
