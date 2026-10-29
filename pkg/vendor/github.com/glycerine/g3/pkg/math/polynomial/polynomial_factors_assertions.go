package polynomial

// Source: src/math/polynomial/polynomial.cpp:1633,1676.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Both source predicates contain only caller-supplied scalar values. Source
// object/index reads remain after the assertion and its actual debugger action.
func (f *PolynomialFactors) sourceScalarAssert(line int32, expression string, condition bool) error {
	if !polynomialSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled || condition {
		return e
	}
	f.assertionRunning = true
	defer func() { f.assertionRunning = false }()
	if e := diagnostic.DebugAssertionFailure(d, "src/math/polynomial/polynomial.cpp", line, expression); e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}
