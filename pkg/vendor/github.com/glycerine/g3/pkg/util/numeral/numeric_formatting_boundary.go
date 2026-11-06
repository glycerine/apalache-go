package numeral

// Source: src/util/mpz.cpp:1385-1410; src/util/mpn.cpp:371-399; src/util/buffer.h.

type IntegerMagnitudeView struct {
	owner *IntegerManager
	limbs limbView
}

func (m *IntegerManager) BorrowMagnitude(a *Int) (IntegerMagnitudeView, error) {
	if e := m.check(a); e != nil {
		return IntegerMagnitudeView{}, e
	}
	if a.Cell == 0 {
		return IntegerMagnitudeView{}, ErrNumericOwner
	}
	c := m.Cell(a.Cell)
	size, e := c.Backing.ReadWord(m.Allocator, 0)
	if e != nil {
		return IntegerMagnitudeView{}, e
	}
	return IntegerMagnitudeView{m, limbView{allocator: m.Allocator, backing: c.Backing, size: size}}, nil
}
func (v IntegerMagnitudeView) Size() uint32 { return v.limbs.size }
func (v IntegerMagnitudeView) WriteCString(s []byte, f *NaturalFormattingScope) error {
	if v.owner == nil || !v.owner.ValidOwner() || f == nil {
		return ErrNumericOwner
	}
	return v.owner.Natural.sourceToCStringScoped(v.limbs, sourceDecimalDestination{character: s}, uint64(len(s)), f)
}
func (s *naturalFormattingScope) Pending() bool       { return s.PendingField }
func (s *naturalFormattingScope) Completed() uint8    { return s.CompletedField }
func (s *naturalFormattingScope) CleanupCause() error { return s.cleanupCause }
func (s *naturalFormattingScope) CleanupAfterSource(m *NaturalManager) error {
	if m == nil || !m.ValidOwner() {
		return ErrNumericOwner
	}
	s.Active = false
	return m.cleanupFormattingScope(s)
}
