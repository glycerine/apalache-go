//go:build g3debug

package numeral

// Source: src/util/mpz.cpp selected Lehmer and ExtendedGCD SASSERT sites.

import "github.com/glycerine/g3/pkg/util/diagnostic"

const sourceGCDDebug = true

func gcdSourceRequire(condition bool) error {
	if !condition {
		return ErrIntegerAssertion
	}
	return nil
}

func gcdSourceRequireAt(condition bool, line int32, expression string) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled || condition {
		return e
	}
	return diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", line, expression)
}

// Actual source ge expression, rather than an arithmetic postcondition oracle.
func (m *IntegerManager) gcdSourceGreaterEqual(a, b *Int) error {
	cmp, e := m.Compare(a, b)
	if e != nil {
		return e
	}
	return gcdSourceRequire(cmp >= 0)
}

func (m *IntegerManager) gcdSourceGreaterEqualAt(a, b *Int, line int32, expression string) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	cmp, e := m.Compare(a, b)
	if e != nil || cmp >= 0 {
		return e
	}
	return diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", line, expression)
}
