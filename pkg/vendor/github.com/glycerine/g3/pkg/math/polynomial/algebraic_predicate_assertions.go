package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:251,260,306,322.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func predicateAssertionsEnabled() (bool, error) {
	if !polynomialSourceDebug {
		return false, nil
	}
	return diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
}
func (m *AlgebraicRootCore) predicateAssertionFailure(line int32, expression string) error {
	if m.predicateDiagnosticRunning {
		return ErrAlgebraicPending
	}
	m.predicateDiagnosticRunning = true
	defer func() { m.predicateDiagnosticRunning = false }()
	e := diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/math/polynomial/algebraic_numbers.cpp", line, expression)
	if e != nil {
		return errors.Join(ErrAlgebraicAssertion, e)
	}
	return nil
}
func (m *AlgebraicRootCore) predicateBasicAssert(a *Algebraic, line int32) error {
	enabled, e := predicateAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if l != nil && l.kind != 0 {
		return m.predicateAssertionFailure(line, "a.is_basic()")
	}
	return nil
}

// The numerical comparison is itself an SASSERT operand. Disabled/erased
// assertions must not allocate, inspect numerals or acquire a child receipt.
func (m *AlgebraicRootCore) predicateUpperAssert(upper *unumeral.BinaryRational, rational bool) error {
	enabled, e := predicateAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	var less bool
	if rational {
		less, e = m.lessPredicateRational(upper)
	} else {
		less, e = m.lessPredicateInteger(upper)
	}
	if e != nil {
		return e
	}
	if less {
		line := int32(260)
		if rational {
			line = 322
		}
		return m.predicateAssertionFailure(line, "bqm().ge(upper(c), candidate)")
	}
	return nil
}

// Source cpp704: preserve the || operand boundary and debugger action.
func (m *AlgebraicRootCore) magnitudeSignAssert(lower, upper *unumeral.BinaryRational) error {
	return m.magnitudeSignAssertWithCleanup(lower, upper, false)
}
func (m *AlgebraicRootCore) magnitudeSignAssertWithCleanup(lower, upper *unumeral.BinaryRational, cleanup bool) error {
	enabled, e := predicateAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	var nonnegative bool
	if cleanup {
		nonnegative, e = m.config.Binary.IsNonNegForCleanup(lower)
	} else {
		nonnegative, e = m.config.Binary.IsNonNeg(lower)
	}
	if e != nil || nonnegative {
		return e
	}
	var nonpositive bool
	if cleanup {
		nonpositive, e = m.config.Binary.IsNonPosForCleanup(upper)
	} else {
		nonpositive, e = m.config.Binary.IsNonPos(upper)
	}
	if e != nil || nonpositive {
		return e
	}
	return m.predicateAssertionFailure(704, "bqm().is_nonneg(l) || bqm().is_nonpos(u)")
}
