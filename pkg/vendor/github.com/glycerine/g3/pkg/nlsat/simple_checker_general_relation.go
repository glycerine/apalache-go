package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1439-1474.

func simpleSignsCompatible(actual, want simpleSign) bool {
	if actual == simpleNone {
		return true
	}
	switch actual {
	case simpleEQ:
		return want != simpleLT && want != simpleGT
	case simpleGE:
		return want != simpleLT
	case simpleGT:
		return want != simpleLT && want != simpleLE && want != simpleEQ
	case simpleLE:
		return want != simpleGT
	case simpleLT:
		return want != simpleGT && want != simpleGE && want != simpleEQ
	}
	return true
}
