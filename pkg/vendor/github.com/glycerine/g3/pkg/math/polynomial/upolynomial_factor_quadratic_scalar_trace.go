package polynomial

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
)

func (q *upolyFactorFrontendScope) scalarTracePending() bool {
	for i := range q.scalarTrace {
		if q.scalarTrace[i].Active() {
			return true
		}
	}
	return false
}
func (q *upolyFactorFrontendScope) scalarTraceBlocker() error {
	for i := range q.scalarTrace {
		if q.scalarTrace[i].Active() {
			if e := q.scalarTrace[i].SourceCause(); e != nil {
				return e
			}
		}
	}
	return nil
}

// An unjoined body failure does not stop older source destructors. Permit
// only this actual parent's source unwind, never output/runtime reentry.
func (q *upolyFactorFrontendScope) scalarTraceSourceReadAllowed() bool {
	if !q.actual() || !q.active || !(q.sourceExecuting || q.quadratic.sourceExecuting || q.vectorNormalize.sourceExecuting) {
		return false
	}
	m := q.service.manager
	return m.callbackDepth == 0 && !m.config.Integer.RuntimeOwner().FailureTransportActive() && !m.config.Integer.BorrowTraceTags().OutputCallbackActive()
}
func (q *upolyFactorFrontendScope) cleanupScalarTrace() error {
	m := q.service.manager
	if q.executing || m.factorTrace != nil && m.factorTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	for i := len(q.scalarTrace) - 1; i >= 0; i-- {
		r := &q.scalarTrace[i]
		if r.Active() {
			if e := m.modular.CleanupSourceString(r); e != nil {
				return e
			}
		}
	}
	if m.pendingFactorScalarTrace == q && !q.scalarTracePending() {
		m.pendingFactorScalarTrace = nil
	}
	return nil
}

// Source: upolynomial.cpp:2974,2990. Values are formatted completely into
// native Go strings before the outer trace stream receives each operand.
func (s *UPolynomialFactorizationService) traceFactorQuadraticScalars(q *upolyFactorFrontendScope, values [3]*unumeral.Int, line int) error {
	m := s.manager
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("factor") {
		return nil
	}
	if q == nil || !q.actual() || !q.active || !q.executing || q.service != s {
		return ErrUPolynomialOwner
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	captions := [3]string{"a: ", "\nb: ", "\nc: "}
	count := 3
	if line == 2990 {
		captions = [3]string{"disc_sqrt: "}
		count = 1
	}
	var computation error
	_ = tags.EmitConditional("factor", "factor_2_sqf_pp", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i := 0; i < count; i++ {
			_ = linearWriteText(out, captions[i])
			text, e := m.modular.SourceStringTracked(values[i], &q.scalarTrace[i])
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
