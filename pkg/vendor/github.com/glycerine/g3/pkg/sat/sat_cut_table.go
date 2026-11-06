package sat

func (c satCut) tableMask() uint64 {
	return uint64(1)<<(uint64(1)<<c.size) - 1
}

func (c *satCut) SetTable(table uint64) { c.table = table & c.tableMask() }
func (c *satCut) Negate()               { c.SetTable(^c.table) }
func (c satCut) Table() uint64          { return (c.table | c.dontCare) & c.tableMask() }
func (c satCut) NTable() uint64         { return (^c.table | c.dontCare) & c.tableMask() }
func (c satCut) DontCare() uint64       { return c.dontCare }
func (c *satCut) AddDontCare(t uint64)  { c.dontCare |= t }
func (c satCut) IsTrue() bool           { return c.tableMask() & ^c.Table() == 0 }
func (c satCut) IsFalse() bool          { return c.tableMask() & ^c.dontCare & c.table == 0 }

func satCutEffectMask(i uint32) uint64 {
	if i == 6 {
		return ^uint64(0)
	}
	m := uint64(1)<<(uint64(1)<<i) - 1
	for w := uint32(1) << (i + 1); w < 64; w *= 2 {
		m |= m << w
	}
	return m
}

func (c *satCut) RemoveElem(i uint32) bool {
	if i >= c.size {
		return false
	}
	copy(c.elems[i:], c.elems[i+1:c.size])
	c.size--
	c.elems[c.size] = 0
	m := satCutEffectMask(i)
	var table uint64
	for j, offset := uint32(0), uint32(0); j < 64; j++ {
		if m&(uint64(1)<<j) != 0 {
			table |= ((c.table >> j) & 1) << offset
			offset++
		}
	}
	c.table, c.dontCare, c.filter = table, 0, 0
	for j := uint32(0); j < c.size; j++ {
		c.filter |= uint32(1) << (c.elems[j] & 31)
	}
	return true
}

func satCutTableString(numInput uint32, table uint64) string {
	n := uint32(1) << numInput
	b := make([]byte, n)
	for i := uint32(0); i < n; i++ {
		b[i] = '0' + byte(table>>i&1)
	}
	return string(b)
}
