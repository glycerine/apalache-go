package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:12-28,248-260.

type simpleSign uint8

const (
	simpleEQ simpleSign = iota
	simpleLT
	simpleGT
	simpleNone
	simpleLE
	simpleGE
	simpleNEQ
)

func simpleLiteralSign(kind AtomKind, negated bool) (simpleSign, bool) {
	var sign simpleSign
	switch kind {
	case AtomEQ:
		sign = simpleEQ
	case AtomLT:
		sign = simpleLT
	case AtomGT:
		sign = simpleGT
	default:
		return simpleNone, false
	}
	if negated {
		switch sign {
		case simpleEQ:
			sign = simpleNEQ
		case simpleLT:
			sign = simpleGE
		case simpleGT:
			sign = simpleLE
		}
	}
	return sign, true
}

func (s simpleSign) flipped() simpleSign {
	switch s {
	case simpleLT:
		return simpleGT
	case simpleGT:
		return simpleLT
	case simpleLE:
		return simpleGE
	case simpleGE:
		return simpleLE
	}
	return s
}
