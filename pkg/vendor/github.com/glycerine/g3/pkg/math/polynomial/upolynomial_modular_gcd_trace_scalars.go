package polynomial

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Actual default mod_gcd lexical NUMs, constructed at source declarations.
// Persistent Go backing preserves warm allocations without inventing source
// member construction. Source and owning cursors remain separate on failure.
type upolyModGCDTraceScalars struct {
	invocation                uint64
	parent                    *upolySquareFreeCallScope
	parentInvocation          uint64
	values                    [6]unumeral.Int
	constructed, ownRemaining uint8
	executing                 bool
	bodyCause, sourceCause    error
}

func (q *upolyModGCDTraceScalars) finish(m *UPolynomialManager, body error) error {
	q.bodyCause = body
	if q.parent != nil && q.parent.valid() && q.parentInvocation == q.parent.invocation {
		q.sourceCause = q.parent.numericSourceCause()
		if q.sourceCause == nil {
			q.sourceCause = q.parent.image.sourceStop
		}
		if q.sourceCause == nil {
			q.sourceCause = q.parent.euclidTrace.sourceBlocker()
		}
		if q.sourceCause == nil {
			q.sourceCause = q.parent.modGCDTrace.sourceBlocker()
		}
		if q.sourceCause == nil {
			q.sourceCause = q.parent.exactTrace.sourceBlocker()
		}
	} else {
		q.sourceCause = m.euclidTrace.sourceBlocker()
	}
	if q.sourceCause == nil {
		q.sourceCause = m.modGCDTrace.sourceBlocker()
	}
	if q.sourceCause == nil && m.pendingCRATrace != nil && m.pendingCRATrace.pending() {
		q.sourceCause = m.pendingCRATrace.sourceFailure()
		if q.sourceCause == nil {
			q.sourceCause = ErrUPolynomialCleanupPending
		}
	}
	if q.sourceCause == nil && m.pendingExactDivTrace != nil {
		q.sourceCause = m.pendingExactDivTrace.sourceBlocker()
	}

	if q.sourceCause == nil {
		for q.constructed > 0 {
			if _, e := upolynomialSourceNumeralDel(m.config.Integer, &q.values[q.constructed-1]); e != nil {
				q.sourceCause = e
				break
			}
			q.constructed--
		}
	}
	q.ownRemaining = q.constructed
	if q.sourceCause == nil {
		return body
	}
	return errors.Join(body, q.sourceCause)
}
func (m *UPolynomialManager) CleanupModularGCDTrace() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	if m.modGCDScalars.executing || m.factorTrace != nil && m.factorTrace.executing || m.displayExecuting() {
		return ErrUPolynomialCleanupPending
	}
	m.modGCDScalars.executing = true
	defer func() { m.modGCDScalars.executing = false }()
	if e := m.modGCDTrace.cleanupOwned(); e != nil {
		return e
	}
	if m.pendingCRATrace != nil && m.pendingCRATrace.pending() {
		return ErrUPolynomialCleanupPending
	}
	if m.pendingExactDivTrace != nil && m.displayReceiptPending(m.pendingExactDivTrace) {
		return ErrUPolynomialCleanupPending
	}
	if m.pendingEuclidTrace != nil && m.displayReceiptPending(m.pendingEuclidTrace) {
		return ErrUPolynomialCleanupPending
	}
	q := &m.modGCDScalars
	for q.ownRemaining > 0 {
		if e := m.config.Integer.CleanupRawNumeral(&q.values[q.ownRemaining-1]); e != nil {
			return e
		}
		q.ownRemaining--
	}
	return nil
}
