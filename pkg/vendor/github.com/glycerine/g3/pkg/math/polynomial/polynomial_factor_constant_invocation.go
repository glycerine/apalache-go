package polynomial

// Source: src/math/polynomial/polynomial.cpp:6577-6590;
// src/math/polynomial/polynomial.h:153-157; src/util/scoped_numeral.h:31-33.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialFactorConstantInvocationReceipt struct {
	manager    *PolynomialFactors
	frame      *polynomialFactorConstantScope
	invocation uint64
}
type polynomialFactorConstantScope struct {
	invocation                                     uint64
	historyIndex                                   int
	historyRecorded                                bool
	active, executing, finishing, localConstructed bool
	kind, phase                                    uint8
	numeric                                        polynomialNumericRoots
	normalize                                      [3]unumeral.IntegerNormalizeInvocationReceipt
	traceNumeral                                   unumeral.NumeralInsertionReceipt
	direct                                         polynomialFactorConstantInvocationReceipt
	bodyCause, sourceCause, cleanupCause           error
}
type polynomialFactorConstantHistory struct {
	invocation                           uint64
	kind                                 uint8
	normalize                            [3]unumeral.IntegerNormalizeInvocationReceipt
	traceNumeral                         unumeral.NumeralInsertionReceipt
	bodyCause, sourceCause, cleanupCause error
}

func (f *PolynomialFactors) matchesFactorConstantInvocation(r *polynomialFactorConstantInvocationReceipt) bool {
	return f != nil && f.self == f && f.manager != nil && f.manager.self == f.manager && f.manager.core != nil && f.manager.core.self == f.manager.core && r != nil && r.manager == f && r.frame == &f.constantInvocation && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (f *PolynomialFactors) factorConstantDiagnostic(r *polynomialFactorConstantInvocationReceipt) (polynomialFactorConstantHistory, bool) {
	if f.matchesFactorConstantInvocation(r) {
		s := r.frame
		return polynomialFactorConstantHistory{s.invocation, s.kind, s.normalize, s.traceNumeral, s.bodyCause, s.sourceCause, s.cleanupCause}, true
	}
	if f == nil || f.self != f || f.manager == nil || f.manager.self != f.manager || f.manager.core == nil || f.manager.core.self != f.manager.core || r == nil || r.manager != f || r.frame != &f.constantInvocation || r.invocation == 0 {
		return polynomialFactorConstantHistory{}, false
	}
	for i := range f.constantHistory {
		if f.constantHistory[i].invocation == r.invocation {
			return f.constantHistory[i], true
		}
	}
	return polynomialFactorConstantHistory{}, false
}
func (f *PolynomialFactors) archiveFactorConstantHistory() error {
	s := &f.constantInvocation
	if s.historyRecorded {
		return nil
	}
	s.historyIndex = len(f.constantHistory)
	s.historyRecorded = true
	f.constantHistory = append(f.constantHistory, polynomialFactorConstantHistory{s.invocation, s.kind, s.normalize, s.traceNumeral, s.bodyCause, s.sourceCause, s.cleanupCause})
	return nil
}
func (f *PolynomialFactors) beginFactorConstant(kind uint8, r *polynomialFactorConstantInvocationReceipt) (*polynomialFactorConstantScope, error) {
	if e := f.open(); e != nil {
		return nil, e
	}
	s := &f.constantInvocation
	if s.active || s.executing || s.finishing || s.invocation == ^uint64(0) || f.manager.core.callbackDepth != 0 {
		return nil, ErrPolynomialPending
	}
	*s = polynomialFactorConstantScope{invocation: s.invocation + 1, kind: kind, active: true, executing: true}
	s.direct = polynomialFactorConstantInvocationReceipt{f, s, s.invocation}
	if r != nil {
		*r = s.direct
	}
	return s, nil
}
func numericIntegerManager_factorConstantContextRoots(m *unumeral.IntegerManager, c integerAllocationInvocationContext) *polynomialNumericRoots {
	r := c.factorConstant
	if m == nil || !m.ValidOwner() || r == nil || r.manager == nil || !r.manager.matchesFactorConstantInvocation(r) || c.factorConstantInvocation == 0 || c.factorConstantInvocation != r.invocation || r.manager.manager.core.integer != m || !r.frame.active || !r.frame.executing || r.manager.manager.core.callbackDepth != 0 {
		return nil
	}
	return &r.frame.numeric
}
func (f *PolynomialFactors) factorConstantNumericContext() integerAllocationInvocationContext {
	s := &f.constantInvocation
	return integerAllocationInvocationContext{factorConstant: &s.direct, factorConstantInvocation: s.invocation}
}
func (f *PolynomialFactors) factorConstantPending() bool {
	s := &f.constantInvocation
	if s.traceNumeral.Active() || polynomialNegNumericPending(&s.numeric) {
		return true
	}
	for i := range s.normalize {
		if f.manager.core.modular.MatchesPNormalizeInvocation(&s.normalize[i]) {
			return true
		}
	}
	return false
}
func (f *PolynomialFactors) finishFactorConstantSource() error {
	s := &f.constantInvocation
	if s.sourceCause != nil {
		return s.sourceCause
	}
	s.finishing = true
	defer func() { s.finishing = false; f.updateFactorConstantHistory() }()
	if e := s.traceNumeral.SourceCause(); e != nil {
		s.sourceCause = e
		return e
	}
	if s.localConstructed {
		if _, e := upolynomialSourceNumeralDel(f.manager.core.integer, &f.local); e != nil {
			s.sourceCause = e
			return e
		}
		s.localConstructed = false
		f.localActive = false
	}
	if !f.factorConstantPending() {
		s.active = false
	}
	return nil
}
func (f *PolynomialFactors) cleanupFactorConstantInvocation(r *polynomialFactorConstantInvocationReceipt, abort bool) (cause error) {
	if !f.matchesFactorConstantInvocation(r) {
		return nil
	}
	s := r.frame
	if !s.active {
		return nil
	}
	if s.executing || s.finishing || f.manager.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	s.finishing = true
	defer func() {
		s.finishing = false
		if cause != nil && s.cleanupCause == nil {
			s.cleanupCause = cause
		}
		f.updateFactorConstantHistory()
	}()
	if s.traceNumeral.Active() {
		if e := s.traceNumeral.Complete(f.manager.core.integer.Natural, false); e != nil {
			return e
		}
		if e := s.traceNumeral.Complete(f.manager.core.integer.Natural, true); e != nil {
			return e
		}
	}
	for i := len(s.normalize) - 1; i >= 0; i-- {
		if e := f.manager.core.modular.CleanupPNormalizeInvocation(&s.normalize[i], false); e != nil {
			return e
		}
	}
	// This is separate owning acknowledgment of the helper's actual scoped NUM.
	if s.localConstructed {
		if e := f.manager.core.integer.Del(&f.local); e != nil {
			return e
		}
		s.localConstructed = false
		f.localActive = false
	}
	if abort {
		// Unjoined child allocation roots are drained only after the helper's
		// actual older scoped NUM has completed its owning acknowledgment.
		for i := len(s.normalize) - 1; i >= 0; i-- {
			if e := f.manager.core.modular.CleanupPNormalizeInvocation(&s.normalize[i], true); e != nil {
				return e
			}
		}
		if e := s.numeric.cleanup(f.manager.core.integer); e != nil {
			return e
		}
	}
	if !f.factorConstantPending() {
		s.active = false
	}
	return nil
}
func (m *MultivariatePolynomialManager) accumulateFactorConstantWithReceipt(f *PolynomialFactors, a *unumeral.Int, r *polynomialFactorConstantInvocationReceipt) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if f == nil || f.manager != m {
		return ErrPolynomialOwner
	}
	s, e := f.beginFactorConstant(1, r)
	if e != nil {
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = errors.Join(cause, f.finishFactorConstantSource())
	}()
	if e = m.traceAccumulateFactorConstant(f, a, &s.traceNumeral); e != nil {
		return e
	}
	f.local = unumeral.Int{}
	f.localActive = true
	s.localConstructed = true
	return f.invoke(func() error {
		if e := numericIntegerModularManager_mulWithFactorConstantReceipt(m.core.modular, &f.constant, a, &f.local, &s.direct); e != nil {
			return e
		}
		s.phase = 1
		return numericIntegerModularManager_copyWithFactorConstantReceipt(m.core.modular, &f.constant, &f.local, &s.direct)
	})
}
func (m *MultivariatePolynomialManager) flipFactorSignWithReceipt(f *PolynomialFactors, r *polynomialFactorConstantInvocationReceipt) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if f == nil || f.manager != m {
		return ErrPolynomialOwner
	}
	s, e := f.beginFactorConstant(2, r)
	if e != nil {
		return e
	}
	f.local = unumeral.Int{}
	f.localActive = true
	s.localConstructed = true
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = errors.Join(cause, f.finishFactorConstantSource())
	}()
	return f.invoke(func() error {
		if e := numericIntegerModularManager_copyWithFactorConstantReceipt(m.core.modular, &f.local, &f.constant, &s.direct); e != nil {
			return e
		}
		s.phase = 1
		if e := numericIntegerModularManager_negWithFactorConstantReceipt(m.core.modular, &f.local, &s.direct); e != nil {
			return e
		}
		s.phase = 2
		return numericIntegerModularManager_copyWithFactorConstantReceipt(m.core.modular, &f.constant, &f.local, &s.direct)
	})
}
func (f *PolynomialFactors) setConstantWithReceipt(a *unumeral.Int, r *polynomialFactorConstantInvocationReceipt) (cause error) {
	s, e := f.beginFactorConstant(0, r)
	if e != nil {
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = errors.Join(cause, f.finishFactorConstantSource())
	}()
	return f.invoke(func() error {
		return numericIntegerModularManager_copyWithFactorConstantReceipt(f.manager.core.modular, &f.constant, a, &s.direct)
	})
}

func (f *PolynomialFactors) factorConstantInvocationSourceCause(r *polynomialFactorConstantInvocationReceipt) error {
	h, ok := f.factorConstantDiagnostic(r)
	if !ok {
		return nil
	}
	return h.sourceCause
}

// This is a lifecycle refusal only; acquisition remains exclusively in the
// explicitly passed concrete receipt at the actual numerical leaf.
func (m *MultivariatePolynomialCore) factorConstantExecuting() bool {
	if m == nil || m.self != m {
		return false
	}
	f := m.factorsOwner
	return f != nil && f.self == f && f.manager != nil && f.manager.core == m && ((f.matchesFactorConstantInvocation(&f.constantInvocation.direct) && (f.constantInvocation.executing || f.constantInvocation.finishing)) || (f.matchesFactorMemberSet(&f.memberSet.direct) && (f.memberSet.executing || f.memberSet.finishing)))
}

func numericIntegerModularManager_factorConstantAllocationContext(m *unumeral.IntegerModularManager, r *polynomialFactorConstantInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || !m.ValidOwner() || r == nil || r.manager == nil || !r.manager.matchesFactorConstantInvocation(r) || r.manager.manager.core.modular != m || !r.frame.active || !r.frame.executing || r.manager.manager.core.callbackDepth != 0 {
		return integerAllocationInvocationContext{}
	}
	return r.manager.factorConstantNumericContext()
}
func numericIntegerModularManager_copyWithFactorConstantReceipt(m *unumeral.IntegerModularManager, out, a *unumeral.Int, r *polynomialFactorConstantInvocationReceipt) error {
	context := numericIntegerModularManager_factorConstantAllocationContext(m, r)
	if r != nil {
		if context.factorConstant == nil {
			return ErrPolynomialOwner
		}
		if factorNumeralCallExecuting(m.Integers()) {
			return ErrPolynomialPending
		}
	}
	if context.factorConstant != nil {
		if e := context.factorConstant.frame.numeric.operation.enter(m.Integers()); e != nil {
			return e
		}
		defer context.factorConstant.frame.numeric.operation.leave()
	}
	return m.WithAllocationContext((context).utility(m.Integers())).Copy(out, a)
}
func numericIntegerModularManager_mulWithFactorConstantReceipt(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, r *polynomialFactorConstantInvocationReceipt) error {
	context := numericIntegerModularManager_factorConstantAllocationContext(m, r)
	if r != nil {
		if context.factorConstant == nil {
			return ErrPolynomialOwner
		}
		if factorNumeralCallExecuting(m.Integers()) {
			return ErrPolynomialPending
		}
	}
	if context.factorConstant != nil {
		if e := context.factorConstant.frame.numeric.operation.enter(m.Integers()); e != nil {
			return e
		}
		defer context.factorConstant.frame.numeric.operation.leave()
	}
	return m.WithAllocationContext((context).utility(m.Integers())).Mul(a, b, out)
}
func numericIntegerModularManager_negWithFactorConstantReceipt(m *unumeral.IntegerModularManager, out *unumeral.Int, r *polynomialFactorConstantInvocationReceipt) error {
	context := numericIntegerModularManager_factorConstantAllocationContext(m, r)
	if r != nil {
		if context.factorConstant == nil {
			return ErrPolynomialOwner
		}
		if factorNumeralCallExecuting(m.Integers()) {
			return ErrPolynomialPending
		}
	}
	if context.factorConstant != nil {
		if e := context.factorConstant.frame.numeric.operation.enter(m.Integers()); e != nil {
			return e
		}
		defer context.factorConstant.frame.numeric.operation.leave()
	}
	return m.WithAllocationContext((context).utility(m.Integers())).Neg(out)
}
