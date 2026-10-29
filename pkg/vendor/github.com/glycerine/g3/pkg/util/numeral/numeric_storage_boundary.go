package numeral

// Source: src/util/mpz.h, mpq.h and mpbq.h: numeral fields in source aggregates.

// NativeWords encodes a borrowed numeral for pointer-free source aggregate
// storage. It transfers no ownership. The owning manager must outlive the
// aggregate, and a decoded numeral is validated by that manager before use.
func (a Int) NativeWords() [3]uint32 {
	return [3]uint32{uint32(a.Val), uint32(a.Cell), uint32(uint64(a.Cell) >> 32)}
}

// IntFromNativeWords decodes the same borrowed source representation.
func IntFromNativeWords(w [3]uint32) Int {
	return Int{Val: int32(w[0]), Cell: NumericCellRef(uint64(w[1]) | uint64(w[2])<<32)}
}

// NativeScalar and NativeCellToken are the fields of the borrowed source
// storage codec. The token retains its generation and carries no ownership.
func (a Int) NativeScalar() int32     { return a.Val }
func (a Int) NativeCellToken() uint64 { return uint64(a.Cell) }

// BorrowNumerator and BorrowDenominator expose the source member addresses.
// They acquire no new numeral ownership; the containing value and its manager
// must remain alive throughout the borrow.
func (a *Rational) BorrowNumerator() *Int              { return &a.Num }
func (a *Rational) BorrowDenominator() *Int            { return &a.Den }
func (a *BinaryRational) BorrowNumerator() *Int        { return &a.Num }
func (a *BinaryInterval) BorrowLower() *BinaryRational { return &a.lower }
func (a *BinaryInterval) BorrowUpper() *BinaryRational { return &a.upper }

// These constructors reconstruct borrowed source aggregates. They do not copy
// numerals, normalize them, or acquire authority to destroy them.
func RationalFromBorrowedNumerals(num, den Int) Rational { return Rational{Num: num, Den: den} }
func BinaryRationalFromBorrowedNumeral(num Int, k uint32) BinaryRational {
	return BinaryRational{Num: num, KField: k}
}
func BinaryIntervalFromBorrowedBounds(lower, upper BinaryRational) BinaryInterval {
	return BinaryInterval{lower: lower, upper: upper}
}

// IsFailureNumeral recognizes only this owner's permanently dead source
// allocation descriptors. Ordinary numeric reads continue to reject them.
func (m *IntegerManager) IsFailureNumeral(a *Int) bool {
	return m.ValidOwner() && a != nil && m.IsFailureRef(a.Cell)
}

// AcknowledgeFailedNumeral clears only the captured poison token; it cannot
// destroy a live numeral or clear a foreign replacement in the same slot.
func (m *IntegerManager) AcknowledgeFailedNumeral(a *Int, expected uint64) bool {
	if !m.IsFailureNumeral(a) || uint64(a.Cell) != expected {
		return false
	}
	a.Cell = 0
	return true
}

// Move implements mpz's source move constructor: retain the scalar bits in
// the moved-from value while transferring its owning cell to the result.
func (a *Int) Move() Int { result := *a; a.Cell = 0; return result }
func (m *IntegerManager) IsLiveCellToken(token uint64) bool {
	return m.ValidOwner() && uint64(NumericCellRef(token)) == token && ValidNumericRef(m.Cells, NumericCellRef(token))
}
