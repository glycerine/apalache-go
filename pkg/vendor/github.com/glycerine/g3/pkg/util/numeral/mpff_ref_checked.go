//go:build g3checked

package numeral

// Checked generation adaptation.

type ffRef uint64

func ffPack(index, generation uint32, negative bool) ffRef {
	v := index
	if negative {
		v |= 1 << 31
	}
	return ffRef(uint64(generation)<<32 | uint64(v))
}
func ffGeneration(r ffRef) uint32 { return uint32(uint64(r) >> 32) }
