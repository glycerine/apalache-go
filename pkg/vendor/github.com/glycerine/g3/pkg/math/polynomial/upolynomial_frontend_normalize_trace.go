package polynomial

// Source: src/math/polynomial/upolynomial.cpp:391-392; src/util/mpzzp.h:145-148.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
)

// The retained frontend owns the actual character-formatting receipts. Native
// Go strings replace ostringstream and std::string; no STL state is modeled.
func (m *UPolynomialManager) traceFrontendNormalization(q *upolyFactorFrontendScope, old, g, x, tmp *unumeral.Int) error {
	if !polynomialSourceDebug {
		return nil
	}
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("div_bug") {
		return nil
	}
	if q == nil || !q.actual() || !q.active || !q.executing || q.service.manager != m {
		return ErrUPolynomialOwner
	}
	if q.scalarTracePending() {
		return ErrUPolynomialCleanupPending
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	var computation error
	_ = tags.EmitConditional("div_bug", "normalize", "src/math/polynomial/upolynomial.cpp", 391, func() bool {
		equal, e := m.modular.Equal(tmp, old)
		computation = e
		return e == nil && !equal
	}, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i, a := range [4]*unumeral.Int{old, g, x, tmp} {
			_ = linearWriteText(out, [4]string{"old(p[i]): ", ", g: ", ", p[i]: ", ", tmp: "}[i])
			text, e := m.modular.SourceStringTracked(a, &q.scalarTrace[i])
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, text)
		}
		_ = linearWriteText(out, "\n")
		return nil
	})
	if q.scalarTracePending() {
		m.pendingFactorScalarTrace = q
	}
	return computation
}
