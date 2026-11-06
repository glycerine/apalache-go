//go:build !g3debug

package numeral

// Source MPBQ_DEBUG absent.

func (m *BinaryRationalManager) debugSelected(l, u, r *BinaryRational) error { return nil }
func (m *BinaryRationalManager) debugEnclosure(qm *RationalManager, q *Rational, l, u *BinaryRational) error {
	return nil
}
func (m *BinaryRationalManager) debugRationalEqual(a *BinaryRational, qm *RationalManager, q *Rational) error {
	return nil
}

func binaryCheckDiagnostics(z *IntegerManager, s *RationalService) error { return nil }
func (m *BinaryRationalManager) debugAssert(ok bool) error               { return nil }
func (m *BinaryRationalManager) debugValue(a *BinaryRational) (OwnedRational, error) {
	return OwnedRational{}, nil
}
func (m *BinaryRationalManager) debugInteger(a *Int) (OwnedRational, error) {
	return OwnedRational{}, nil
}
func (m *BinaryRationalManager) debugDel(a *OwnedRational) error { return nil }
func (m *BinaryRationalManager) debugEqualValue(old *OwnedRational, a *BinaryRational) error {
	return nil
}
func (m *BinaryRationalManager) debugArithmetic(a, b *OwnedRational, out *BinaryRational, op uint8, traceLine int) error {
	return nil
}
func (m *BinaryRationalManager) debugApproxSnapshot(a, old *BinaryRational) error    { return nil }
func (m *BinaryRationalManager) debugApproxDrop(old *BinaryRational) error           { return nil }
func (m *BinaryRationalManager) debugApprox(old, a *BinaryRational, plus bool) error { return nil }
