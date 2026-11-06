package sat

import "math"

const satCutMaxSize = 5

type satCutVal struct {
	t, f uint64
}

type satCut struct {
	filter   uint32
	size     uint32
	elems    [satCutMaxSize]uint32
	table    uint64
	dontCare uint64
}

func newSATCut(id uint32) satCut {
	return satCut{filter: uint32(1) << (id & 31), size: 1, elems: [satCutMaxSize]uint32{id}, table: 2}
}

func (c satCut) Size() uint32   { return c.size }
func (c satCut) Filter() uint32 { return c.filter }
func (c satCut) Elem(i uint32) uint32 {
	if i >= c.size {
		return math.MaxUint32
	}
	return c.elems[i]
}

func (c *satCut) Add(id uint32) bool {
	if c.size >= satCutMaxSize {
		return false
	}
	c.elems[c.size] = id
	c.size++
	c.filter |= uint32(1) << (id & 31)
	return true
}

func (c satCut) DomainEqual(other satCut) bool {
	if c.size != other.size {
		return false
	}
	for i := uint32(0); i < c.size; i++ {
		if c.elems[i] != other.elems[i] {
			return false
		}
	}
	return true
}

func (c satCut) Equal(other satCut) bool {
	return c.DomainEqual(other) && c.Table() == other.Table()
}

func (c satCut) SubsetOf(other satCut) bool {
	if other.filter != c.filter|other.filter {
		return false
	}
	i := uint32(0)
	otherID := other.Elem(i)
	for j := uint32(0); j < c.size; j++ {
		id := c.elems[j]
		for id > otherID {
			i++
			otherID = other.Elem(i)
		}
		if id != otherID {
			return false
		}
		i++
		otherID = other.Elem(i)
	}
	return true
}
