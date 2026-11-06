package sat

type satLUTFound func(uint64, []BoolVar, BoolVar)

type satLUTFinder struct {
	maxSize         uint32
	masks           [7]uint64
	combination     uint64
	numCombinations uint32
	vars            []BoolVar
	missing         []uint32
	onLUT           satLUTFound
}

func newSATLUTFinder(f satLUTFound) *satLUTFinder {
	l := &satLUTFinder{maxSize: 5, onLUT: f}
	for i := uint32(0); i <= 6; i++ {
		l.masks[i] = satCutEffectMask(i)
	}
	return l
}

type satLUTClause struct {
	Literals []Literal
	Learned  bool
}
type satLUTBinary struct {
	First, Second Literal
	Learned       bool
}
