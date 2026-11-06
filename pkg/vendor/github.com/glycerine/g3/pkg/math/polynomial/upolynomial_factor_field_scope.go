package polynomial

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

const (
	factorFieldSquare uint8 = iota + 1
	factorFieldWhole
)

// One frame mirrors one live source call. FactorZP nests a square-free frame
// above its local factor collection, so a stopped child is cleaned first.
type upolyFactorFieldScope struct {
	m                                  *UPolynomialManager
	parent                             *upolyFactorFieldScope
	kind                               uint8
	active, executing, sourceExecuting bool
	ownExecuting                       bool
	polynomial                         upolyDisplayInvocationReceipt
	factors                            upolyFactorsDisplayReceipt
	vectors                            [7]upolyVectorState
	constant                           unumeral.Int
	constructed, sourceStage, ownStage uint8
	sourceIndex, ownIndex              uint32
	sourceStarted, ownStarted          bool
	sq                                 *UPolynomialFactors
	bodyCause, sourceCause, ownCause   error
}

func (m *UPolynomialManager) beginFactorField(kind uint8) (*upolyFactorFieldScope, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	var q *upolyFactorFieldScope
	for i := range m.factorFieldFrames {
		if !m.factorFieldFrames[i].active {
			q = &m.factorFieldFrames[i]
			break
		}
	}
	if q == nil {
		return nil, ErrUPolynomialCleanupPending
	}
	*q = upolyFactorFieldScope{m: m, parent: m.factorField, kind: kind, active: true, executing: true}
	m.factorField = q
	return q, nil
}

func (q *upolyFactorFieldScope) traceBlocker() error {
	if e := q.factors.sourceBlocker(); e != nil {
		return e
	}
	if q.m.displayReceiptPending(&q.polynomial) {
		if e := q.m.displayReceiptSourceCause(&q.polynomial); e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	return nil
}

func (q *upolyFactorFieldScope) vector(stage uint8) *upolyVectorState {
	// Source declarations: div_tmp, T_0, constant, T_0_d, T, V, W, A_ek.
	if stage == 1 {
		return &q.vectors[0]
	}
	if stage == 2 {
		return &q.vectors[1]
	}
	if stage >= 4 && stage <= 8 {
		return &q.vectors[stage-3]
	}
	return nil
}

func (q *upolyFactorFieldScope) finish(body error) error {
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	q.executing = false
	if q.m.factorField != q {
		// A retained nested square-free call is the younger source blocker.
		return errors.Join(q.bodyCause, ErrUPolynomialCleanupPending)
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	if q.sourceCause == nil {
		q.sourceCause = q.traceBlocker()
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{q.m}, q.m.modular.BorrowTraceTags()); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	defer borrow.unbind()
	if q.kind == factorFieldWhole {
		if q.sq != nil {
			if e := q.sq.Close(); e != nil {
				q.sourceCause = e
				return errors.Join(q.bodyCause, e)
			}
			q.sq = nil
		}
	} else {
		if !q.sourceStarted {
			q.sourceStage, q.sourceStarted = q.constructed, true
		}
		for q.sourceStage != 0 {
			if q.sourceStage == 3 {
				if _, e := upolynomialSourceNumeralDel(q.m.config.Integer, &q.constant); e != nil {
					q.sourceCause = e
					return errors.Join(q.bodyCause, e)
				}
				q.sourceStage--
				continue
			}
			v := q.vector(q.sourceStage)
			for q.sourceIndex < v.size {
				if _, e := upolynomialSourceNumeralDel(q.m.config.Integer, &q.m.values(v)[q.sourceIndex]); e != nil {
					q.sourceCause = e
					return errors.Join(q.bodyCause, e)
				}
				q.sourceIndex++
			}
			v.size = 0
			q.m.freeBacking(v)
			q.sourceIndex = 0
			q.sourceStage--
		}
	}
	q.active = false
	q.m.factorField = q.parent
	return q.bodyCause
}

func (q *upolyFactorFieldScope) cleanup() (cause error) {
	if !q.active || q.executing || q.sourceExecuting || q.ownExecuting {
		if q.active {
			return ErrUPolynomialCleanupPending
		}
		return nil
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	if e := q.factors.cleanup(); e != nil {
		return e
	}
	if e := q.m.cleanupDisplayReceipt(&q.polynomial); e != nil {
		return e
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{q.m}, q.m.modular.BorrowTraceTags()); e != nil {
		return e
	}
	defer borrow.unbind()
	if q.kind == factorFieldWhole {
		if q.sq != nil {
			if e := q.sq.Close(); e != nil {
				return e
			}
			q.sq = nil
		}
	} else {
		if !q.ownStarted {
			q.ownStage, q.ownIndex, q.ownStarted = q.sourceStage, q.sourceIndex, true
			if !q.sourceStarted {
				q.ownStage = q.constructed
			}
		}
		for q.ownStage != 0 {
			if q.ownStage == 3 {
				if e := q.m.config.Integer.CleanupRawNumeral(&q.constant); e != nil {
					return e
				}
				q.ownStage--
				continue
			}
			v := q.vector(q.ownStage)
			for q.ownIndex < v.size {
				if e := q.m.config.Integer.CleanupRawNumeral(&q.m.values(v)[q.ownIndex]); e != nil {
					return e
				}
				q.ownIndex++
			}
			v.size = 0
			q.m.freeBacking(v)
			q.ownIndex = 0
			q.ownStage--
		}
	}
	q.active = false
	q.m.factorField = q.parent
	return nil
}

// CleanupFactorFieldFactorization owns only retained finite-field call frames.
func (m *UPolynomialManager) CleanupFactorFieldFactorization() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	for m.factorField != nil {
		if e := m.factorField.cleanup(); e != nil {
			return e
		}
	}
	return nil
}
