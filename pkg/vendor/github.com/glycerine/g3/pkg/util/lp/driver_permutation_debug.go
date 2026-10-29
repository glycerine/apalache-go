//go:build g3debug

package lp

// Execute the actual dense-matrix product/equality from the source debug body.
func driverCheckPermutationProduct(p, l *FloatPermutation, original []uint32) error {
	clone, err := FloatPermutationFromValues(5, original)
	if err != nil {
		return err
	}
	reverse, err := l.Inverse()
	if err != nil {
		return err
	}
	product, err := MultiplyFloatDebugMatrices(clone, reverse)
	if err != nil {
		return err
	}
	equal, err := FloatDebugMatricesEqual(p, product)
	if err != nil {
		return err
	}
	if !equal {
		return ErrAssertion
	}
	return nil
}
