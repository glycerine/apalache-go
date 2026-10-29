package polynomial

// Source: src/math/polynomial/polynomial.cpp:5095,5104,5120.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func polynomialExactDivSourceAssert(line int32, expression string, predicate func() (bool, error)) error {
	if !polynomialSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	ok, e := predicate()
	if e != nil || ok {
		return e
	}
	if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/polynomial.cpp", line, expression); e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}

// VERIFY evaluates its condition in every profile. A Continue action cannot
// safely resume this site because the next source statement consumes the null
// quotient, so the Go port retains the typed undefined-domain stop.
func polynomialExactDivSourceVerifyFailure() error {
	e := diagnostic.BorrowProcessDebugContext().VerifyFailure(
		"src/math/polynomial/polynomial.cpp", 5142, "div(m_r, m_q, m_r_q)",
	)
	if polynomialSourceDebug && e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	if e != nil {
		return errors.Join(ErrPolynomialUnreachable, e)
	}
	return ErrPolynomialUnreachable
}
