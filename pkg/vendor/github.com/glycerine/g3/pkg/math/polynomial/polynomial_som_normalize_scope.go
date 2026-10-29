package polynomial

// Source: src/math/polynomial/polynomial.cpp:1863-1876.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Ordinary lexical destruction never acknowledges a stopped source pointer.
// The owning branch is reached only by explicit caller cleanup/owner close.

func (b *polynomialSOM) finishNormalizeNumeral(owning bool) error {
	return b.finishNormalizeNumeralPhase(owning, owning)
}
func (b *polynomialSOM) finishNormalizeNumeralPhase(owning, abort bool) (err error) {
	m := b.owner
	s := &b.normalizeScope
	if !m.actualSOMNormalizeBuffer(b) {
		return ErrPolynomialOwner
	}
	if !b.gcdActive {
		if !owning {
			return b.gcdCleanupCause
		}
		return nil
	}
	if m == nil || m.self != m || !m.hasSOMNormalizeInvocationHistory(&s.receipt) || s.receipt.buffer != b || s.receipt.frame != s {
		return ErrPolynomialOwner
	}
	if s.executing || s.finishing || m.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if !owning && b.gcdCleanupCause != nil {
		return b.gcdCleanupCause
	}
	if !b.gcdActive {
		return nil
	}
	s.finishing = true
	defer func() {
		s.finishing = false
		if owning && err != nil && s.ownCause == nil {
			s.ownCause = err
		}
	}()
	if !owning {
		if e := numericIntegerManager_somNormalizeNumericSourceCause(m.integer, &s.receipt); e != nil {
			b.gcdCleanupCause = e
			s.sourceCause = e
			return e
		}
	}
	if owning {
		if e := numericIntegerManager_cleanupSOMNormalizationNumeric(m.integer, &s.receipt, false); e != nil {
			return e
		}
	}
	if (!owning && !b.gcdEnded) || (owning && !s.ownEnded) {
		var e error
		if owning {
			e = m.deleteNumeral(&b.gcd)
		} else {
			_, e = upolynomialSourceNumeralDel(m.integer, &b.gcd)
		}
		if e != nil {
			if !owning && b.gcdCleanupCause == nil {
				b.gcdCleanupCause = e
				s.sourceCause = e
			}
			return e
		}
		if owning {
			s.ownEnded = true
		} else {
			b.gcdEnded = true
		}
	}
	if owning && abort {
		if e := numericIntegerManager_cleanupSOMNormalizationNumeric(m.integer, &s.receipt, true); e != nil {
			return e
		}
	}
	pending := polynomialNegNumericPending(&b.gcdNumeric) || m.integer.MatchesRawInvocation(&s.gcdRaw) || m.integer.MatchesRawInvocation(&s.divisionRaw) || m.integer.MatchesRawInvocation(&s.extendedRaw) || m.modular.MatchesPNormalizeInvocation(&s.normalize)

	if !pending {
		b.gcdActive = false
		s.active = false
	}
	return nil
}

func (b *polynomialSOM) beginNormalizeNumeral(parent *polynomialNormalizeScope) error {
	m := b.owner
	if !m.actualSOMNormalizeBuffer(b) {
		return ErrPolynomialOwner
	}
	if m.callbackDepth != 0 || b.normalizeScope.finishing {
		return ErrPolynomialPending
	}
	if b.gcdActive || (m.hasSOMNormalizeInvocationHistory(&m.somNormalizationOwner) && (m.somNormalizationOwner.frame.active || m.somNormalizationOwner.frame.finishing)) {
		return ErrPolynomialPending
	}
	if b.gcdInvocation == ^uint64(0) {
		return arena.ErrBounds
	}
	b.gcdInvocation++
	b.normalizeScope = polynomialSOMNormalizeScope{active: true, invocation: b.gcdInvocation}
	b.normalizeScope.receipt = polynomialSOMNormalizeInvocationReceipt{manager: m, buffer: b, frame: &b.normalizeScope, invocation: b.gcdInvocation}
	m.somNormalizationOwner = b.normalizeScope.receipt
	b.gcdNormalization = parent
	if parent != nil {
		parent.somInvocation = b.gcdInvocation
		parent.som = b.normalizeScope.receipt
	}
	b.gcdSubstitution = nil
	if b == &m.som && m.substitution.scope.executing && m.callbackDepth == 0 {
		b.gcdSubstitution = &m.substitution.scope
		m.substitution.scope.somInvocation = b.gcdInvocation
		m.substitution.scope.som = b.normalizeScope.receipt
	}
	b.gcd = unumeral.Int{}
	b.gcdActive = true
	b.gcdBodyCause = nil
	b.gcdCleanupCause = nil
	b.gcdEnded = false
	b.gcdGeneration = m.pscExecutingOwner()
	return nil
}
