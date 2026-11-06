package hilbert

import "sort"

func (b *Basis) resolveOrdered(left, right *vector) (*vector, error) {
	return b.resolve(left, right)
}

func orderBasis(values []*vector) error {
	var result error
	sort.SliceStable(values, func(i, j int) bool {
		left, leftErr := vectorMagnitude(values[i])
		right, rightErr := vectorMagnitude(values[j])
		if leftErr != nil {
			result = leftErr
		}
		if rightErr != nil {
			result = rightErr
		}
		return left < right
	})
	return result
}
