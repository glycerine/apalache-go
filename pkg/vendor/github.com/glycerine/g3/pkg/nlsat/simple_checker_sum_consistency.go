package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:795-953.

func simpleSumConsistency(term simpleSign, cSign int, want simpleSign, count int) (actionable, conflict bool, adjusted simpleSign) {
	adjusted = want
	switch term {
	case simpleEQ:
		if cSign < 0 {
			return false, want == simpleEQ || want == simpleGE || want == simpleGT, adjusted
		}
		if cSign == 0 {
			return false, want == simpleGT || want == simpleLT, adjusted
		}
		return false, want == simpleEQ || want == simpleLE || want == simpleLT, adjusted
	case simpleLT:
		if cSign <= 0 {
			return false, want == simpleEQ || want == simpleGE || want == simpleGT, adjusted
		}
		if want == simpleLE || want == simpleLT {
			return false, false, adjusted
		}
		if count > 1 && want == simpleEQ {
			adjusted = simpleGE
		}
		return true, false, adjusted
	case simpleLE:
		if cSign < 0 {
			return false, want == simpleEQ || want == simpleGE || want == simpleGT, adjusted
		}
		if cSign == 0 {
			return false, want == simpleGT, adjusted
		}
		if want == simpleLE || want == simpleLT {
			return false, false, adjusted
		}
		if count > 1 && want == simpleEQ {
			adjusted = simpleGE
		}
		return true, false, adjusted
	case simpleGT:
		if cSign < 0 {
			if want == simpleGE || want == simpleGT {
				return false, false, adjusted
			}
			if count > 1 && want == simpleEQ {
				adjusted = simpleLE
			}
			return true, false, adjusted
		}
		return false, want == simpleEQ || want == simpleLE || want == simpleLT, adjusted
	case simpleGE:
		if cSign < 0 {
			if want == simpleGE || want == simpleGT {
				return false, false, adjusted
			}
			if count > 1 && want == simpleEQ {
				adjusted = simpleLE
			}
			return true, false, adjusted
		}
		if cSign == 0 {
			return false, want == simpleLT, adjusted
		}
		return false, want == simpleEQ || want == simpleLE || want == simpleLT, adjusted
	}
	return false, false, adjusted
}
