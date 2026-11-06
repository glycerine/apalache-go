package arith

// Source: src/tactic/arith/linear_equation.cpp:179-193,93-97.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// The two combine preconditions are source SASSERTs, so a release build
// proceeds with a zero multiplier instead of rejecting the call.
func (l *LinearEquationManager) assertNonzeroEquationMultiplier(b *numeral.Int, line int32, condition string) error {
	d := diagnostic.BorrowProcessDebugContext()
	on, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !on {
		return err
	}
	zero, err := l.manager.Integers().IsZero(b)
	if err != nil || !zero {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/tactic/arith/linear_equation.cpp", line, condition)
}

// Source: linear_equation.cpp:60. The rational constructor's minimum arity
// is a debug assertion; a release build can construct a normalized singleton.
func (l *LinearEquationManager) assertRationalEquationArity(size int) error {
	d := diagnostic.BorrowProcessDebugContext()
	on, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !on || size > 1 {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/tactic/arith/linear_equation.cpp", 60, "sz > 1")
}

// The normalized constructor's DEBUG_CODE marks each input variable before
// sorting. If an assertion action aborts, earlier marks remain as in source.
func (l *LinearEquationManager) assertUniqueNormalizedEquationVars(xs []uint32) error {
	d := diagnostic.BorrowProcessDebugContext()
	on, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !on {
		return err
	}
	for _, x := range xs {
		if x == noEquationPosition || uint64(x)+1 > uint64(int(^uint(0)>>1)) {
			return arena.ErrBounds
		}
		if int(x) >= len(l.mark) {
			l.mark = append(l.mark, make([]bool, int(x)+1-len(l.mark))...)
		}
		if l.mark[x] {
			if err := diagnostic.DebugAssertionFailure(d, "src/tactic/arith/linear_equation.cpp", 184, "!m_mark[x]"); err != nil {
				return err
			}
		}
		l.mark[x] = true
	}
	for _, x := range xs {
		l.mark[x] = false
	}
	return nil
}

// The source checks this invariant only inside DEBUG_CODE, before the first
// mk_core TRACE record or numeral operation.
func (l *LinearEquationManager) assertSortedEquationVars(xs []uint32) error {
	d := diagnostic.BorrowProcessDebugContext()
	on, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !on {
		return err
	}
	for i := 1; i < len(xs); i++ {
		if xs[i-1] >= xs[i] {
			return diagnostic.DebugAssertionFailure(d, "src/tactic/arith/linear_equation.cpp", 95, "xs[i-1] < xs[i]")
		}
	}
	return nil
}
