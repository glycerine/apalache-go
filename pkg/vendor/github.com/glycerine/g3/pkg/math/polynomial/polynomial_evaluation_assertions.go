package polynomial

// Source: src/math/polynomial/polynomial.cpp:6440-6492,6516,6532,6536.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func polynomialEvaluationSourceAssert(line int32, expression string, predicate func() (bool, error)) error {
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

func (m *MultivariatePolynomialManager) evaluationCoreSourceAssertions(p Polynomial, start, end, x uint32) error {
	if e := polynomialEvaluationSourceAssert(6440, "start < end", func() (bool, error) {
		return start < end, nil
	}); e != nil {
		return e
	}
	if e := polynomialEvaluationSourceAssert(6441, "end <= p->size()", func() (bool, error) {
		n, e := m.core.readWord(p, 8)
		return end <= n, e
	}); e != nil {
		return e
	}
	return polynomialEvaluationSourceAssert(6442, "is_valid(x)", func() (bool, error) {
		return m.core.monomials.IsValidVar(x)
	})
}

func (m *MultivariatePolynomialManager) evaluationConstantSourceAssert(p Polynomial) error {
	return polynomialEvaluationSourceAssert(6516, "size(p)==1", func() (bool, error) {
		n, e := m.core.readWord(p, 8)
		return n == 1, e
	})
}
