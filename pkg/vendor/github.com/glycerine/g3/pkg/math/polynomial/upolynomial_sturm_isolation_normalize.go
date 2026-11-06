package polynomial

// Source: upolynomial.cpp:366-401,2469-2474; outer Sturm isolation normalization.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (q *upolySturmIsolationScope) normalizePending() bool {
	s := &q.normalization
	for i := range s.insertions {
		if s.insertions[i].Active() {
			return true
		}
	}
	return s.constructed != 0 && !s.owned || q.owner.config.Integer.MatchesCopyAllocationInvocation(&s.copy) || q.owner.modular.MatchesPNormalizeInvocation(&s.normalize)
}

func (q *upolySturmIsolationScope) finishNormalization(until uint8, cause error) error {
	s, z := &q.normalization, q.owner.config.Integer
	for i := range s.insertions {
		if s.sourceStop == nil {
			s.sourceStop = s.insertions[i].SourceCause()
		}
	}
	if s.sourceStop == nil {
		for i := len(q.raw) - 1; i >= 0; i-- {
			if stop := z.RawInvocationSourceCause(&q.raw[i]); stop != nil {
				s.sourceStop = stop
				break
			}
		}
	}
	if s.sourceStop == nil {
		q.finishing = true
		defer func() { q.finishing = false }()
		for s.constructed > until {
			if e := unumeral.SourceIntegerDel(z, &s.values[s.constructed-1]); e != nil {
				s.sourceStop = e
				break
			}
			s.constructed--
		}
	}
	if s.sourceStop != nil && q.childStop == nil {
		q.childStop = s.sourceStop
	}
	return errors.Join(cause, s.sourceStop)
}

func (q *upolySturmIsolationScope) normalizeView(v UPolynomialView) (cause error) {
	m := q.owner
	if q.context().sturmIsolationFrame(m.config.Integer) != q {
		return ErrUPolynomialOwner
	}
	// These early returns precede the source g declaration.
	if v.count <= 1 {
		return m.normalizeWithSourceContext(v, q.context())
	}
	if e := q.beginNormalization(); e != nil {
		return e
	}
	s := &q.normalization
	defer func() { cause = q.endNormalization(cause) }()
	g := &s.values[0]
	if e := m.gcdSpanWithSourceContext(v, g, q.context()); e != nil {
		return e
	}
	one, e := m.modular.IsOne(g)
	if e != nil || one {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		if e := q.normalizeCoefficient(v, i); e != nil {
			return e
		}
	}
	return nil
}

func (q *upolySturmIsolationScope) beginNormalization() error {
	if q.context().sturmIsolationFrame(q.owner.config.Integer) != q {
		return ErrUPolynomialOwner
	}
	s := &q.normalization
	if q.normalizePending() || s.executing || s.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	*s = upolySturmNormalizeScope{invocation: s.invocation + 1, constructed: 1, executing: true}
	return nil
}
func (q *upolySturmIsolationScope) endNormalization(cause error) error {
	s := &q.normalization
	s.executing = false
	s.bodyCause = cause
	return q.finishNormalization(0, cause)
}

func (q *upolySturmIsolationScope) normalizeCoefficient(v UPolynomialView, i uint32) (cause error) {
	m, s := q.owner, &q.normalization
	g, old, tmp := &s.values[0], &s.values[1], &s.values[2]
	if polynomialSourceDebug {
		s.values[1] = unumeral.Int{}
		s.constructed = 2
		defer func() { cause = q.finishNormalization(1, cause) }()
		x, e := m.read(v, i)
		if e != nil {
			return e
		}
		if e = m.modular.CopyFromOwnerTracked(old, m.config.Integer, x, &s.copy, &s.normalize); e != nil {
			return e
		}
	}
	x, e := m.read(v, i)
	if e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext(q.context().utility(m.config.Integer)).Div(x, g, x); e != nil {
		return e
	}
	if polynomialSourceDebug {
		s.values[2] = unumeral.Int{}
		s.constructed = 3
		if e = m.modular.WithAllocationContext(q.context().utility(m.config.Integer)).Mul(g, x, tmp); e != nil {
			return e
		}
		if e = q.traceNormalization(x); e != nil {
			return e
		}
		return m.normalizationCoefficientAssert(tmp, old)
	}
	return nil
}

func (q *upolySturmIsolationScope) cleanupNormalization() error {
	s, z := &q.normalization, q.owner.config.Integer
	for i := len(s.insertions) - 1; i >= 0; i-- {
		if s.insertions[i].Active() {
			if e := s.insertions[i].Complete(z.Naturals(), false); e != nil {
				return e
			}
		}
	}
	if e := q.owner.modular.CleanupPNormalizeInvocation(&s.normalize, true); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}
	if !s.owning {
		s.ownRemaining = s.constructed
		s.owning = true
	}
	for s.ownRemaining > 0 {
		if e := z.CleanupRawNumeral(&s.values[s.ownRemaining-1]); e != nil {
			return e
		}
		s.ownRemaining--
	}
	for i := len(s.insertions) - 1; i >= 0; i-- {
		if s.insertions[i].Active() {
			if e := s.insertions[i].Complete(z.Naturals(), true); e != nil {
				return e
			}
		}
	}
	s.owned = true
	return nil
}
