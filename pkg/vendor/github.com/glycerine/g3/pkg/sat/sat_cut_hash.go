package sat

import uhash "github.com/glycerine/g3/pkg/util/hash"

func (c satCut) Hash() uint32 {
	if c.size == 0 {
		return 11
	}
	return uhash.Composite(uint32(c.Table()), c.size, func(i uint32) uint32 { return c.elems[i] })
}

func (c satCut) DomainHash() uint32 {
	if c.size == 0 {
		return 11
	}
	return uhash.Composite(3, c.size, func(i uint32) uint32 { return c.elems[i] })
}
