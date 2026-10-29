package numeral

// Source: src/util/mpz.cpp/mpn.cpp; diagnostics of actual numeric storage.

type NumericStorageStats struct {
	LiveCells, CellSlots, RetiredCells, CachedCells uint32
}

func (z *IntegerManager) StorageStats() (s NumericStorageStats) {
	cells := z.Cells.Stats()
	s.LiveCells, s.CellSlots, s.RetiredCells = cells.Live, cells.Slots, cells.Retired
	for _, ref := range z.Tmp {
		if ValidNumericRef(z.Cells, ref) {
			s.CachedCells++
		}
	}
	for _, ref := range z.Arg {
		if ValidNumericRef(z.Cells, ref) {
			s.CachedCells++
		}
	}
	for _, a := range [2]Int{z.IntMin, z.Two64} {
		if ValidNumericRef(z.Cells, a.Cell) {
			s.CachedCells++
		}
	}
	return s
}
func (m *IntegerModularManager) OwnedNumeralCells() (count uint32) {
	for _, a := range [7]Int{m.P, m.Lower, m.Upper, m.InvTmp1, m.InvTmp2, m.InvTmp3, m.DivTmp} {
		if ValidNumericRef(m.manager.Cells, a.Cell) {
			count++
		}
	}
	if m.PowerPending && ValidNumericRef(m.manager.Cells, m.PowerTemporary.Cell) {
		count++
	}
	return count
}
func (m *IntegerManager) InterruptedNumeral(a *Int) bool {
	if a == nil || !m.ValidOwner() || a.Cell == 0 || !ValidNumericRef(m.Cells, a.Cell) {
		return false
	}
	c := m.Cell(a.Cell)
	return c.CleanupPending || c.Released
}
