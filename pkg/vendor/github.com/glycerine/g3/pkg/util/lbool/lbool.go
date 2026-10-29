package lbool

// Source: src/util/lbool.h; src/util/lbool.cpp.

// LBool preserves false, undefined, and true as distinct source values.
type LBool int32

const (
	False LBool = -1
	Undef LBool = 0
	True  LBool = 1
)

func (l LBool) Negated() LBool { return -l }

func FromBool(b bool) LBool {
	if b {
		return True
	}
	return False
}

func (l LBool) String() string {
	switch l {
	case False:
		return "l_false"
	case True:
		return "l_true"
	default:
		return "l_undef"
	}
}

func (l LBool) SATString() string {
	switch l {
	case False:
		return "unsatisfiable"
	case True:
		return "satisfiable"
	default:
		return "unknown"
	}
}
