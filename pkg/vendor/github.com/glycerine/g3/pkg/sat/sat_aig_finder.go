package sat

import z3rand "github.com/glycerine/g3/pkg/util/random"

type satAIGFound func(Literal, []Literal) error
type satITEFound func(Literal, Literal, Literal, Literal) error

type satAIGFinder struct {
	big      *satBIG
	binaries map[[2]Literal]bool
	onAIG    satAIGFound
	onITE    satITEFound
}

func newSATAIGFinder(onAIG satAIGFound, onITE satITEFound) *satAIGFinder {
	return &satAIGFinder{big: newSATBIG(z3rand.New(0)), binaries: make(map[[2]Literal]bool), onAIG: onAIG, onITE: onITE}
}

func satAIGPair(a, b Literal) [2]Literal {
	if a > b {
		a, b = b, a
	}
	return [2]Literal{a, b}
}
