package sat

// Source: src/sat/sat_big.h; src/sat/sat_big.cpp.

import z3rand "github.com/glycerine/g3/pkg/util/random"

// satBIG is the source binary implication graph. The adjacency list stores
// implications u -> v, corresponding to binary clauses (~u or v).
type satBIG struct {
	random             *z3rand.Generator
	dag                [][]Literal
	roots              []bool
	left, right        []int32
	root, parent       []Literal
	deleted            [][]Literal
	numVars            uint32
	learned            bool
	includeCardinality bool
}

func newSATBIG(random *z3rand.Generator) *satBIG {
	if random == nil {
		random = z3rand.New(0)
	}
	return &satBIG{random: random}
}

func (b *satBIG) numLiterals() uint32 { return b.numVars * 2 }

func (b *satBIG) validLiteral(l Literal) bool {
	return l != DefaultLiteral() && l.index() < b.numLiterals()
}

func (b *satBIG) Learned() bool { return b.learned }
