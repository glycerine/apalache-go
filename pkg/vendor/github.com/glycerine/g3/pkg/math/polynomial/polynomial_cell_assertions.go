package polynomial

// Source: src/math/polynomial/polynomial.cpp:1371,1391,1415,1508,1511-1512.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *MultivariatePolynomialCore) cellSourceAssert(line int32, expression string, predicate func() (bool, error)) error {
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

func (m *MultivariatePolynomialCore) cellIndexAssert(p Polynomial, i uint32, line int32) error {
	return m.cellSourceAssert(line, "idx < size()", func() (bool, error) {
		n, e := m.readWord(p, 8)
		return i < n, e
	})
}

func (m *MultivariatePolynomialCore) constructorMonomial(buffer *polynomialSOM, ms []Monomial, i uint32) (Monomial, error) {
	if buffer != nil {
		return buffer.ms.monomial(m, i)
	}
	if uint64(i) >= uint64(len(ms)) {
		return Monomial{}, arena.ErrBounds
	}
	return ms[i], nil
}
