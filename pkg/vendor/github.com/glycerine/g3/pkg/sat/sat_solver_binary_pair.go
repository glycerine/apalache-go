package sat

// Source: src/sat/sat_solver.h:286,445; src/sat/sat_solver.cpp:2818-2838.

// BinaryLiteralPair keeps both complete literal indices. Collection order is
// the source watch traversal order; no learned bit is packed into either value.
type BinaryLiteralPair struct{ first, second uint32 }

func newSATSolverBinPair(a, b Literal) BinaryLiteralPair {
	return BinaryLiteralPair{a.index(), b.index()}
}
func (p BinaryLiteralPair) Literal1() Literal { return Literal(p.first) }
func (p BinaryLiteralPair) Literal2() Literal { return Literal(p.second) }
