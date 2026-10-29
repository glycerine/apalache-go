package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:513-568;
// src/math/polynomial/upolynomial_factorization_int.h:42-52.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// This is the actual conditional child scope, embedded in the charged pair frame.
// Its RAW transport has no source destructor; only publication or owning abort
// consumes it. The numeral br is scoped and precedes the first related manager.

type upolyHenselCheckScope struct {
	br, transport                              unumeral.Int
	transportBound                             bool
	brCore, bCore                              *UPolynomialManager
	brPrevious, bPrevious                      *upolyHenselPairScope
	test1, test2a, test2b, test3a, test3b      upolyVectorState
	constructed, sourceRemaining, ownRemaining uint8
	sourceStarted, ownStarted, sourceEnded     bool
	sourceIndex, ownIndex                      uint32
	sourceExecuting, ownExecuting              bool
	bodyCause, sourceCause                     error
	managerStage                               [2]uint8
	managerIndex                               [2]uint32
}

func (h *upolyHenselPairScope) checkVector(i uint8) *upolyVectorState {
	switch i {
	case 3:
		return &h.check.test1
	case 5:
		return &h.check.test2a
	case 6:
		return &h.check.test2b
	case 7:
		return &h.check.test3a
	case 8:
		return &h.check.test3b
	}
	return nil
}
func (h *upolyHenselPairScope) attachCheckManager(m *UPolynomialManager, first bool) {
	if first {
		h.check.brPrevious = m.henselPairCaller
	} else {
		h.check.bPrevious = m.henselPairCaller
	}
	m.henselPairCaller = h
}
func (h *upolyHenselPairScope) finishHenselCheckSource(body error) error {
	c := &h.check
	if c.bodyCause == nil {
		c.bodyCause = body
	}
	if !c.sourceStarted {
		c.sourceStarted = true
		c.sourceRemaining = c.constructed
	}
	if c.sourceCause == nil {
		c.sourceCause = h.traceSourceBlocker()
	}
	if c.sourceCause == nil {
		c.sourceCause = h.numericSourceCause()
	}
	if c.sourceCause != nil || c.sourceEnded {
		return errors.Join(c.bodyCause, c.sourceCause)
	}
	c.sourceExecuting = true
	defer func() { c.sourceExecuting = false }()
	for c.sourceRemaining > 0 {
		var e error
		switch c.sourceRemaining {
		case 1:
			_, e = upolynomialSourceNumeralDel(h.service.manager.config.Integer, &c.br)
		case 2:
			e = c.brCore.sourceFinalizeWithHensel(&h.receipt)
		case 4:
			e = c.bCore.sourceFinalizeWithHensel(&h.receipt)
		default:
			e = upolynomialScopedVectorSourceDel(h.service.manager, h.checkVector(c.sourceRemaining), &c.sourceIndex)
		}
		if e != nil {
			c.sourceCause = e
			if h.sourceCause == nil {
				h.sourceCause = e
			}
			return errors.Join(c.bodyCause, e)
		}
		c.sourceRemaining--
		c.sourceIndex = 0
	}
	c.sourceEnded = true
	return c.bodyCause
}
func (h *upolyHenselPairScope) cleanupHenselCheck(abort bool) error {
	c := &h.check
	z := h.service.manager.config.Integer
	c.ownExecuting = true
	defer func() { c.ownExecuting = false }()
	if !c.ownStarted {
		c.ownStarted = true
		c.ownRemaining = c.constructed
	}
	for c.ownRemaining > 0 {
		var e error
		switch c.ownRemaining {
		case 1:
			e = z.CleanupRawNumeral(&c.br)
		case 2:
			e = cleanupHenselRelatedMembers(c.brCore, &c.managerStage[0], &c.managerIndex[0])
		case 4:
			e = cleanupHenselRelatedMembers(c.bCore, &c.managerStage[1], &c.managerIndex[1])
		default:
			v := h.checkVector(c.ownRemaining)
			for c.ownIndex < v.size {
				a := &h.service.manager.values(v)[c.ownIndex]
				if e = z.CleanupRawNumeral(a); e != nil {
					return e
				}
				c.ownIndex++
			}
			v.size = 0
			h.service.manager.freeBacking(v)
		}
		if e != nil {
			return e
		}
		c.ownRemaining--
		c.ownIndex = 0
	}
	if abort {
		if c.transportBound {
			if e := z.CleanupRawNumeral(&c.transport); e != nil {
				return e
			}
			c.transportBound = false
		}

	}
	return nil
}
func (m *UPolynomialManager) toZPCopyWithHensel(p UPolynomialView, out *upolyVectorState, r *upolyHenselPairInvocationReceipt) error {
	if numericIntegerModularManager_henselPairAllocationContext(m.modular, r).henselPairReceipt(m.config.Integer) == nil {
		return ErrUPolynomialOwner
	}
	return m.toZPCopyEngine(p, out, nil, r)
}

func (h *upolyHenselPairScope) releaseHenselCheckManagers() error {
	c := &h.check
	c.ownExecuting = true
	defer func() { c.ownExecuting = false }()
	for _, m := range []*UPolynomialManager{c.bCore, c.brCore} {
		if m != nil && !m.closed {
			if e := m.closeWithHensel(&h.receipt); e != nil {
				return e
			}
		}
	}
	return nil
}
