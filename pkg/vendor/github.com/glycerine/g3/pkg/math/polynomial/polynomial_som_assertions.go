package polynomial

// Source: src/math/polynomial/polynomial.cpp:1871,1898,1910,1919,1936,2149,2234,2248,2270.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// polynomialSOMSourceAssert models SASSERT at the source statement. A Continue
// debugger action returns to the following statement; Throw preserves both the
// polynomial assertion identity and the debugger's concrete cause.
func polynomialSOMSourceAssert(line int32, expression string, condition bool) error {
	return polynomialSOMPredicateSourceAssert(line, expression, func() (bool, error) { return condition, nil })
}

func polynomialSOMPredicateSourceAssert(line int32, expression string, predicate func() (bool, error)) error {
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

// The source uniqueness search is itself inside SASSERT, so disabled
// assertions must not inspect the buffer or its monomial fields.
func (b *polynomialSOM) assertUniqueMonomial(term Monomial, line int32) error {
	expression := "std::find(m_tmp_ms.begin(), m_tmp_ms.end(), m) == m_tmp_ms.end()"
	if line == 2270 {
		expression = "std::find(m_tmp_ms.begin(), m_tmp_ms.end(), m2) == m_tmp_ms.end()"
	}
	return polynomialSOMPredicateSourceAssert(line, expression, func() (bool, error) {
		n, e := b.ms.size(b.owner)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			x, e := b.ms.monomial(b.owner, i)
			if e != nil {
				return false, e
			}
			if x == term {
				return false, nil
			}
		}
		return true, nil
	})
}

func (b *polynomialSOM) assertCompactionMonomial(i uint32, term Monomial) error {
	return polynomialSOMPredicateSourceAssert(1910, "m_tmp_ms[j] != m", func() (bool, error) {
		old, e := b.ms.monomial(b.owner, i)
		return old != term, e
	})
}

func (b *polynomialSOM) assertDiscardedCoefficientZero(i uint32) error {
	return polynomialSOMPredicateSourceAssert(1919, "mng.is_zero(m_tmp_as[i])", func() (bool, error) {
		a, e := b.as.coefficient(b.owner, i)
		if e != nil {
			return false, e
		}
		return b.owner.modular.IsZero(&a)
	})
}
