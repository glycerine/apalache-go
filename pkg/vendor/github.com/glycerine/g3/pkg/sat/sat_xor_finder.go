package sat

type satXORFound func([]Literal) error
type satXORFinder struct {
	maxSize     uint32
	combination uint32
	parity      [6][]bool
	missing     []uint32
	onXOR       satXORFound
}

func newSATXORFinder(f satXORFound) *satXORFinder {
	x := &satXORFinder{maxSize: 5, onXOR: f}
	for n := uint32(0); n <= x.maxSize; n++ {
		p := make([]bool, uint32(1)<<n)
		for a := range p {
			odd := false
			for i := uint32(0); i < n; i++ {
				odd = odd != (uint32(a)&(1<<i) != 0)
			}
			p[a] = odd
		}
		x.parity[n] = p
	}
	return x
}
func (x *satXORFinder) Parity(n, assignment uint32) bool {
	return n <= x.maxSize && assignment < uint32(len(x.parity[n])) && x.parity[n][assignment]
}
