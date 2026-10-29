package polynomial

// Source: src/math/polynomial/polynomial.cpp:6438-6439,6504-6505.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
)

// A diagnostic completes before evaluation proceeds to recursion or return.
// Completed records can reuse the root owner's slot; a pending formatter
// prevents reuse and remains reachable through its actual invocation receipt.
type polynomialEvaluationTraceScope struct {
	self                                 *polynomialEvaluationTraceScope
	owner                                *MultivariatePolynomialManager
	tags                                 *diagnostic.TraceTags
	invocation                           uint64
	active, executing                    bool
	bindings                             [2]unumeral.NumericCallerBinding
	bound                                uint8
	display                              polynomialDisplayInvocationReceipt
	insertions                           [2]unumeral.NumeralInsertionReceipt
	decimal                              algebraicDecimalDisplayInvocationReceipt
	previous                             *polynomialEvaluationTraceScope
	algebraic                            bool
	bodyCause, sourceCause, cleanupCause error
}

func (s *polynomialEvaluationTraceScope) NumericExecuting() bool { return s.executing }
func (m *MultivariatePolynomialCore) evaluationTraceCallback() bool {
	if m == nil || m.self != m || m.evaluationTrace == nil {
		return false
	}
	s := m.evaluationTrace
	if m.callbackDepth != 0 || s.tags.OutputCallbackActive() {
		return true
	}
	rt := m.integer.RuntimeOwner()
	return rt != nil && rt.FailureTransportActive()
}
func (m *MultivariatePolynomialManager) evaluationTraceActual(s *polynomialEvaluationTraceScope) bool {
	return m != nil && m.self == m && s != nil && (s == &m.evaluation.trace || s == &m.algebraicEvaluation.trace) && s.owner == m && s.self == s && s.invocation != 0
}
func (m *MultivariatePolynomialManager) beginEvaluationTrace(s *polynomialEvaluationTraceScope, algebraic bool) (bool, error) {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("eval_bug") {
		return false, nil
	}
	if (!algebraic && s != &m.evaluation.trace) || (algebraic && s != &m.algebraicEvaluation.trace) {
		return false, ErrPolynomialOwner
	}
	if s.active || s.executing {
		return false, ErrPolynomialPending
	}
	if s.invocation == ^uint64(0) {
		return false, arena.ErrBounds
	}
	*s = polynomialEvaluationTraceScope{self: s, owner: m, tags: tags, invocation: s.invocation + 1, active: true, executing: true, previous: m.core.evaluationTrace, algebraic: algebraic}
	m.core.evaluationTrace = s
	if e := m.core.integer.BindCaller(&s.bindings[0], s, 0); e != nil {
		return false, m.finishEvaluationTrace(s, e)
	}
	s.bound = 1
	if e := m.core.integer.Natural.BindCaller(&s.bindings[1], s, 0); e != nil {
		return false, m.finishEvaluationTrace(s, e)
	}
	s.bound = 2
	return true, nil
}
func (m *MultivariatePolynomialManager) finishEvaluationTrace(s *polynomialEvaluationTraceScope, body error) error {
	for s.bound != 0 {
		s.bound--
		if e := s.bindings[s.bound].Unbind(); e != nil {
			panic(e)
		}
	}
	m.core.evaluationTrace = s.previous
	s.previous = nil
	s.executing = false
	s.bodyCause = body
	s.sourceCause = m.core.displayReceiptSourceCause(&s.display)
	for i := range s.insertions {
		if s.sourceCause == nil {
			s.sourceCause = s.insertions[i].SourceCause()
		}
	}
	if vm := s.decimal.owner; vm != nil && s.sourceCause == nil {
		s.sourceCause = vm.decimalDisplayInvocationSourceCause(&s.decimal)
	}
	s.active = m.core.displayReceiptPending(&s.display) || s.insertions[0].Active() || s.insertions[1].Active() || s.sourceCause != nil
	if vm := s.decimal.owner; vm != nil {
		s.active = s.active || vm.decimalDisplayInvocationPending(&s.decimal)
	}
	if s.sourceCause != nil {
		if s.algebraic {
			if m.algebraicEvaluation.sourceCause == nil {
				m.algebraicEvaluation.sourceCause = s.sourceCause
			}
		} else if m.evaluation.sourceCause == nil {
			m.evaluation.sourceCause = s.sourceCause
		}
	}
	return errors.Join(body, s.sourceCause)
}
func (m *MultivariatePolynomialManager) emitEvaluationTrace(s *polynomialEvaluationTraceScope, line int, body func(*polynomialTraceSink) error) error {
	var computation error
	_ = s.tags.EmitConditional("eval_bug", "t_eval_core", "src/math/polynomial/polynomial.cpp", line, func() bool { return true }, func(w io.Writer) error { computation = body(&polynomialTraceSink{writer: w}); return computation })
	return computation
}
func polynomialEvaluationTraceIndices(out *polynomialTraceSink, start, end, x uint32) {
	_ = out.WriteUnsigned(start)
	_ = out.WriteLiteral(", end: ")
	_ = out.WriteUnsigned(end)
	_ = out.WriteLiteral(", x: ")
	_ = out.WriteUnsigned(x)
}
func (m *MultivariatePolynomialManager) traceEvaluationEntry(p Polynomial, start, end, x uint32, s *polynomialEvaluationTraceScope, algebraic bool) (err error) {
	enabled, e := m.beginEvaluationTrace(s, algebraic)
	if e != nil || !enabled {
		return e
	}
	defer func() { err = m.finishEvaluationTrace(s, err) }()
	return m.emitEvaluationTrace(s, 6438, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("p: ")
		if e := m.core.displayPolynomialDirectWithReceipt(p, out, false, &s.display); e != nil {
			return e
		}
		_ = out.WriteLiteral("\nstart: ")
		polynomialEvaluationTraceIndices(out, start, end, x)
		return out.WriteLiteral("\n")
	})
}

func (m *MultivariatePolynomialManager) cleanupEvaluationTrace(s *polynomialEvaluationTraceScope) (err error) {
	if !m.evaluationTraceActual(s) {
		return nil
	}
	if s.executing || m.core.evaluationTraceCallback() {
		return ErrPolynomialPending
	}
	if !s.active {
		return nil
	}
	defer func() {
		if err != nil && s.cleanupCause == nil {
			s.cleanupCause = err
		}
	}()
	if vm := s.decimal.owner; vm != nil {
		if err = vm.cleanupDecimalDisplayInvocation(&s.decimal); err != nil {
			return err
		}
	}
	for i := len(s.insertions) - 1; i >= 0; i-- {
		if err = m.core.integer.CleanupSourceDisplay(&s.insertions[i]); err != nil {
			return err
		}
	}
	if err = m.core.cleanupDisplayReceipt(&s.display); err != nil {
		return err
	}
	s.active = false
	return nil
}
