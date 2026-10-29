package numeral

// Source: src/util/mpf.h:152-153,161,193-194; concrete mpf_manager
// accessors reached by src/api/api_fpa.cpp:933,978-995,1011-1028,1049-1062.

// BorrowSignificand returns the source sig() reference. The returned numeral
// belongs to the float value and must not be deleted or retained after that
// value or manager is released.
func (m *ArbitraryFloatManager) BorrowSignificand(a *ArbitraryFloat) (*Int, error) {
	if err := m.valueGuard(a); err != nil {
		return nil, err
	}
	if err := m.integer.Check(&a.significand); err != nil {
		return nil, err
	}
	return &a.significand, nil
}

// BorrowIntegerManager and BorrowRationalManager return the two actual child
// managers of mpf_manager; callers borrow them until their owner closes.
func (m *ArbitraryFloatManager) BorrowIntegerManager() (*IntegerManager, error) {
	if err := m.valueOwnerGuard(); err != nil {
		return nil, err
	}
	return m.integer, nil
}

func (m *ArbitraryFloatManager) BorrowRationalManager() (*RationalManager, error) {
	if err := m.valueOwnerGuard(); err != nil {
		return nil, err
	}
	return m.rational, nil
}
