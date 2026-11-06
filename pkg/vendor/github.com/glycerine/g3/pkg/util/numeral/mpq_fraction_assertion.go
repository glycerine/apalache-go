package numeral

// Source: src/util/mpq.h:679,690.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func rationalFractionDenominatorAssertion(nonzero bool, line int32) error {
	if !modularSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return e
	}
	if enabled && !nonzero {
		return diagnostic.DebugAssertionFailure(d, "src/util/mpq.h", line, "d != 0")
	}
	return nil
}
