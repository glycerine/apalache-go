package lp

// Source: src/test/lp.cpp:1014-1072, retained --test_perm scenario.

import "io"

// PermutationTestDriver executes the two source permutation compositions and
// the dense/indexed vector application. Results are owned native Go values,
// retained for inspection after Run. Do not mutate them during Run.
type PermutationTestDriver struct {
	Products []*FloatPermutation
	Dense    []float64
	Indexed  *IndexedFloatVector
	started  bool
}

func (d *PermutationTestDriver) Run(out io.Writer) error {
	if d.started {
		return ErrExactVectorPending
	}
	d.started = true
	if err := writeDriverLine(driverReportOutput(out), "test permutations"); err != nil {
		return err
	}
	original := []uint32{4, 2, 0, 3, 1}
	for _, values := range [][]uint32{{1, 2, 3, 4, 0}, {2, 0, 1, 4, 3}} {
		p, err := FloatPermutationFromValues(5, original)
		if err != nil {
			return err
		}
		l, err := FloatPermutationFromValues(5, values)
		if err != nil {
			return err
		}
		if err = p.MultiplyReverseRight(&l.Permutation); err != nil {
			return err
		}
		if err = driverCheckPermutationProduct(p, l, original); err != nil {
			return err
		}
		d.Products = append(d.Products, p)
	}
	d.Dense = []float64{0, 1, 0, 3, 0}
	p, err := FloatPermutationFromValues(5, original)
	if err != nil {
		return err
	}
	d.Indexed = NewIndexedFloatVector(5)
	if err = d.Indexed.SetValue(1, 1); err != nil {
		return err
	}
	if err = d.Indexed.SetValue(3, 3); err != nil {
		return err
	}
	if err = p.ApplyReverseRightToT(d.Dense); err != nil {
		return err
	}
	if err = p.ApplyReverseRightIndexed(d.Indexed); err != nil {
		return err
	}
	if sourceDebugEnabled {
		if !floatPermutationVectorsEqual(d.Dense, d.Indexed.Data) {
			return ErrAssertion
		}
		if !d.Indexed.IsOK() {
			return ErrAssertion
		}
	}
	return nil
}
