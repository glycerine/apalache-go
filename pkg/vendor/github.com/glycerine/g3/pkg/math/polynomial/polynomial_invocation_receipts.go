package polynomial

// Source: src/math/polynomial/polynomial.cpp:3432-3534,4352-4506,5904-5940;
// src/util/obj_ref.h; src/util/scoped_numeral.h.

// These are owning adapter receipts for an actual guarded callee invocation.
// The shared PSC owner generation is deliberately not invocation identity.
// Stable frame address, manager and monotonically increasing frame nonce must
// all match. Erased zero paths acquire no new receipt. Constant SET has only its
// real cold copy continuation; it constructs no ICCP lexical locals. The degree-zero
// ICCP delegate can acquire only its actual Content child, without an ICCP frame.
type polynomialICCPInvocationReceipt struct {
	shortcutContent  polynomialContentInvocationReceipt
	shortcutConstant polynomialICCPConstantInvocationReceipt
	manager          *MultivariatePolynomialManager
	frame            *polynomialICCPScope
	invocation       uint64
}
type polynomialGCDInvocationReceipt struct {
	manager    *MultivariatePolynomialManager
	frame      *polynomialGCDFunctionScope
	invocation uint64
}
type polynomialNormalizeInvocationReceipt struct {
	manager    *MultivariatePolynomialManager
	frame      *polynomialNormalizeScope
	invocation uint64
}

func (m *MultivariatePolynomialManager) hasICCPInvocationHistory(r *polynomialICCPInvocationReceipt) bool {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame == nil || r.invocation == 0 {
		return false
	}
	// The actual owning directory names the guarded frame; copied descriptors
	// never acquire authority over its source children.
	for _, frame := range m.iccpFrames {
		if frame == r.frame {
			return frame.invocation == r.invocation
		}
	}
	return false
}
func (m *MultivariatePolynomialManager) matchesICCPInvocation(r *polynomialICCPInvocationReceipt) bool {
	return m.hasICCPInvocationHistory(r) && r.frame.active
}

// Validate the captured known frame against its actual owning directory before
// reading payload/history. Directory membership validates address, never roots.
func (m *MultivariatePolynomialManager) hasGCDInvocationHistory(r *polynomialGCDInvocationReceipt) bool {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame == nil || r.invocation == 0 {
		return false
	}
	for _, actual := range m.gcdFrames {
		if actual == r.frame {
			return actual.invocation == r.invocation
		}
	}
	return false
}
func (m *MultivariatePolynomialManager) matchesGCDInvocation(r *polynomialGCDInvocationReceipt) bool {
	return m.hasGCDInvocationHistory(r) && r.frame.active
}

func (m *MultivariatePolynomialManager) matchesNormalizeInvocation(r *polynomialNormalizeInvocationReceipt) bool {
	return m.hasNormalizeInvocationHistory(r) && r.frame.active
}
func (m *MultivariatePolynomialManager) iccpInvocationSourceCause(r *polynomialICCPInvocationReceipt) error {
	if r != nil && m.hasICCPConstantHistory(&r.shortcutConstant) {
		return r.shortcutConstant.frame.sourceCause
	}
	if m.hasICCPShortcutContentHistory(r) {
		return m.core.contentInvocationSourceCause(&r.shortcutContent)
	}
	if m.hasICCPInvocationHistory(r) {
		return r.frame.cleanupCause
	}
	return nil
}
func (m *MultivariatePolynomialManager) gcdInvocationSourceCause(r *polynomialGCDInvocationReceipt) error {
	if m.hasGCDInvocationHistory(r) {
		return r.frame.cleanupCause
	}
	return nil
}
func (m *MultivariatePolynomialManager) normalizeInvocationSourceCause(r *polynomialNormalizeInvocationReceipt) error {
	if m.hasNormalizeInvocationHistory(r) {
		return r.frame.sourceCause
	}
	return nil
}
func (m *MultivariatePolynomialManager) cleanupICCPInvocation(r *polynomialICCPInvocationReceipt, abort bool) error {
	if r != nil && m.hasICCPConstantHistory(&r.shortcutConstant) {
		return m.cleanupICCPConstantInvocation(&r.shortcutConstant, abort)
	}
	if m.matchesICCPShortcutContent(r) {
		return m.core.cleanupContentInvocation(&r.shortcutContent, abort)
	}
	if !m.matchesICCPInvocation(r) {
		return nil
	}
	if m.iccpDepth != 0 {
		return ErrPolynomialPending
	}
	return m.cleanupICCPFrame(r.frame, abort)
}
func (m *MultivariatePolynomialManager) cleanupICCPFrame(s *polynomialICCPScope, abort bool) error {
	if s.child != nil && s.child.active && s.child.invocation == s.childInvocation {
		if e := m.cleanupICCPFrame(s.child, abort); e != nil {
			return e
		}
	}
	one := [1]*polynomialICCPScope{s}
	return m.cleanupICCPScopes(one[:], abort)
}
func (m *MultivariatePolynomialManager) cleanupGCDInvocation(r *polynomialGCDInvocationReceipt, abort bool) error {
	if !m.matchesGCDInvocation(r) {
		return nil
	}
	if m.gcdDepth != 0 {
		return ErrPolynomialPending
	}
	return m.cleanupGCDFrame(r.frame, abort)
}
func (m *MultivariatePolynomialManager) cleanupGCDFrame(s *polynomialGCDFunctionScope, abort bool) error {
	if s.child != nil && s.child.active && s.child.invocation == s.childInvocation {
		if e := m.cleanupGCDFrame(s.child, abort); e != nil {
			return e
		}
	}
	one := [1]*polynomialGCDFunctionScope{s}
	return m.cleanupGCDScopes(one[:], abort)
}
func (m *MultivariatePolynomialManager) cleanupNormalizeInvocation(r *polynomialNormalizeInvocationReceipt, abort bool) error {
	if !m.matchesNormalizeInvocation(r) {
		return nil
	}
	return m.cleanupNormalizationPhase(abort)
}

func (m *MultivariatePolynomialManager) matchesICCPShortcutContent(r *polynomialICCPInvocationReceipt) bool {
	return m != nil && m.self == m && m.core != nil && m.core.self == m.core && r != nil && r.manager == m && r.frame == nil && r.invocation == 0 && r.shortcutContent.manager == m.core && m.core.matchesContentInvocation(&r.shortcutContent)
}

func (m *MultivariatePolynomialManager) hasICCPShortcutContentHistory(r *polynomialICCPInvocationReceipt) bool {
	if m == nil || m.self != m || m.core == nil || m.core.self != m.core || r == nil || r.manager != m || r.frame != nil || r.invocation != 0 {
		return false
	}
	c := &r.shortcutContent
	return c.manager == m.core && c.frame == &m.core.contentLocal && c.invocation != 0 && c.invocation == c.frame.invocation
}
