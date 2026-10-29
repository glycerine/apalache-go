package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:583-674;
// src/util/scoped_numeral_vector.h:29-40; src/util/mpzzp.h:97-105.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyHenselPairInvocationReceipt struct {
	service    *UPolynomialFactorizationService
	frame      *upolyHenselPairScope
	invocation uint64
}

// Stable, cached cold storage for the actual lexical declarations. A frame's
// storage is charged once; no Runtime ticket stands in for lexical construction.
type upolyHenselPairScope struct {
	trace                           upolyDisplayInvocationReceipt
	traceBorrow                     upolyFactorTraceBorrow
	inputs                          [5]upolyHenselInputBorrow
	check                           upolyHenselCheckScope
	am                              unumeral.Int
	amConstructed, amEnded, amOwned bool
	amBodyCause, amSourceCause      error
	ownManagerStage                 uint8
	ownManagerIndex                 uint32

	rootPrevious, rmPrevious                   *upolyHenselPairScope
	ownExecuting                               bool
	self                                       *upolyHenselPairScope
	service                                    *UPolynomialFactorizationService
	next                                       *upolyHenselPairScope
	header                                     uint64
	invocation                                 uint64
	receipt                                    upolyHenselPairInvocationReceipt
	active, executing, sourceExecuting         bool
	callDepth                                  uint32
	rm                                         *UPolynomialManager
	f, Vf, t, S, T, tmp                        upolyVectorState
	constructed, sourceRemaining, ownRemaining uint8
	sourceStarted, ownStarted, sourceEnded     bool
	sourceIndex, ownIndex                      uint32
	bodyCause, sourceCause, ownCause           error
	numeric                                    polynomialNumericRoots
	normalize                                  [4]unumeral.IntegerNormalizeInvocationReceipt
	addMul                                     unumeral.IntegerAddMulInvocationReceipt
	divisionRaw, extendedRaw                   unumeral.IntegerRawInvocationReceipt
	boolNumeral                                unumeral.IntegerDividesNumeralInvocationReceipt
}

func (s *UPolynomialFactorizationService) ownsHenselFrame(h *upolyHenselPairScope) bool {
	for f := s.henselPairs; f != nil; f = f.next {
		if f == h {
			return true
		}
	}
	return false
}
func (s *UPolynomialFactorizationService) matchesHenselPairHistory(r *upolyHenselPairInvocationReceipt) bool {
	return s != nil && s.self == s && s.manager != nil && s.manager.self == s.manager && r != nil && r.service == s && r.frame != nil && r.frame.self == r.frame && r.frame.service == s && s.ownsHenselFrame(r.frame) && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (s *UPolynomialFactorizationService) matchesHenselPairInvocation(r *upolyHenselPairInvocationReceipt) bool {
	return s.matchesHenselPairHistory(r) && r.frame.active
}
func (s *UPolynomialFactorizationService) beginHenselPairWithCaller(caller *upolyHenselCallerScope) (upolyHenselPairInvocationReceipt, error) {
	if caller != nil && (!caller.matches() || caller.owner.service != s) {
		return upolyHenselPairInvocationReceipt{}, ErrUPolynomialOwner
	}
	r, e := s.beginHenselPairInvocation()
	if e == nil && caller != nil {
		caller.pair = r
	}
	return r, e
}
func (s *UPolynomialFactorizationService) beginHenselPairInvocation() (upolyHenselPairInvocationReceipt, error) {
	if e := s.checkOwner(); e != nil {
		return upolyHenselPairInvocationReceipt{}, e
	}
	h := s.henselPairs
	for h != nil && h.active {
		h = h.next
	}
	if h == nil {
		n := uint64(reflect.TypeOf(upolyHenselPairScope{}).Size())
		if e := s.manager.config.Integer.Budget().Reserve(n); e != nil {
			return upolyHenselPairInvocationReceipt{}, e
		}
		h = &upolyHenselPairScope{header: n, next: s.henselPairs}
		s.henselPairs = h
	}
	if h.invocation == ^uint64(0) {
		return upolyHenselPairInvocationReceipt{}, arena.ErrSlotLimit
	}
	next, header, nonce := h.next, h.header, h.invocation+1
	*h = upolyHenselPairScope{service: s, next: next, header: header, invocation: nonce, active: true, executing: true}
	h.self = h
	h.receipt = upolyHenselPairInvocationReceipt{s, h, nonce}
	return h.receipt, nil
}
func (h *upolyHenselPairScope) vector(i uint8) (*UPolynomialManager, *upolyVectorState) {
	switch i {
	case 2:
		return h.service.manager, &h.f
	case 3:
		return h.rm, &h.Vf
	case 4:
		return h.rm, &h.t
	case 5:
		return h.rm, &h.S
	case 6:
		return h.rm, &h.T
	case 7:
		return h.rm, &h.tmp
	}
	return nil, nil
}
func (s *UPolynomialFactorizationService) henselPairInvocationSourceCause(r *upolyHenselPairInvocationReceipt) error {
	if !s.matchesHenselPairHistory(r) {
		return nil
	}
	if e := r.frame.traceSourceBlocker(); e != nil {
		return e
	}
	return r.frame.sourceCause
}

// Readonly owning history does not grant live cleanup authority.
func (s *UPolynomialFactorizationService) henselPairInvocationOwnCause(r *upolyHenselPairInvocationReceipt) error {
	if !s.matchesHenselPairHistory(r) {
		return nil
	}
	return r.frame.ownCause
}
func (s *UPolynomialFactorizationService) finishHenselPairSource(r *upolyHenselPairInvocationReceipt, body error) error {
	if !s.matchesHenselPairHistory(r) {
		return ErrUPolynomialOwner
	}
	h := r.frame
	if h.executing || h.sourceExecuting || h.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if h.bodyCause == nil {
		h.bodyCause = body
	}
	if !h.sourceStarted {
		h.sourceStarted = true
		h.sourceRemaining = h.constructed
	}
	if h.sourceCause == nil {
		h.sourceCause = h.traceSourceBlocker()
	}
	if h.sourceCause == nil {
		h.sourceCause = h.numericSourceCause()
	}
	if h.sourceEnded || h.sourceCause != nil {
		return errors.Join(h.bodyCause, h.sourceCause)
	}
	if !h.sourceStarted {
		h.sourceStarted = true
		h.sourceRemaining = h.constructed
	}
	h.enterCall()
	defer h.leaveCall()
	h.sourceExecuting = true
	defer func() { h.sourceExecuting = false }()
	for h.sourceRemaining != 0 {
		var e error
		if h.sourceRemaining == 1 {
			e = h.rm.sourceFinalizeWithHensel(r)
		} else {
			m, v := h.vector(h.sourceRemaining)
			e = upolynomialScopedVectorSourceDel(m, v, &h.sourceIndex)
		}
		if e != nil {
			h.sourceCause = e
			return errors.Join(h.bodyCause, e)
		}
		h.sourceRemaining--
		h.sourceIndex = 0
	}
	h.sourceEnded = true
	return h.bodyCause
}
func (s *UPolynomialFactorizationService) cleanupHenselPairInvocation(r *upolyHenselPairInvocationReceipt, abort bool) (cause error) {
	if !s.matchesHenselPairInvocation(r) {
		return nil
	}
	h := r.frame
	if h.executing || h.sourceExecuting || h.ownExecuting || h.callDepth != 0 {
		return ErrUPolynomialCleanupPending
	}
	defer func() {
		if cause != nil && h.ownCause == nil {
			h.ownCause = cause
		}
	}()
	h.ownExecuting = true
	h.enterCall()
	defer func() { h.leaveCall(); h.ownExecuting = false }()
	if e := s.manager.cleanupDisplayReceipt(&h.trace); e != nil {
		return e
	}
	if e := h.cleanupNumeric(false); e != nil {
		return e
	}
	if e := h.cleanupHenselCheck(false); e != nil {
		return e
	}
	if h.amConstructed && !h.amEnded && !h.amOwned {
		if e := s.manager.config.Integer.CleanupRawNumeral(&h.am); e != nil {
			return e
		}
		h.amOwned = true
	}
	if !h.ownStarted {
		h.ownStarted = true
		h.ownRemaining = h.constructed
	}
	for h.ownRemaining > 1 {
		m, v := h.vector(h.ownRemaining)
		for h.ownIndex < v.size {
			a, e := m.read(m.stateView(v), h.ownIndex)
			if e != nil {
				return e
			}
			if e = m.config.Integer.CleanupRawNumeral(a); e != nil {
				return e
			}
			h.ownIndex++
		}
		v.size = 0
		m.freeBacking(v)
		h.ownIndex = 0
		h.ownRemaining--
	}
	if h.ownRemaining == 1 {
		if e := h.cleanupRelatedSourceMembers(); e != nil {
			return e
		}
	}
	if abort {
		if e := h.cleanupHenselCheck(true); e != nil {
			return e
		}
		if e := h.cleanupNumeric(true); e != nil {
			return e
		}
		if e := h.releaseHenselCheckManagers(); e != nil {
			return e
		}
		if h.ownRemaining == 1 && !h.rm.closed {
			if e := h.rm.closeWithHensel(r); e != nil {
				return e
			}
		}
		h.ownRemaining = 0
		h.active = false
	}
	return nil
}
func (s *UPolynomialFactorizationService) cleanupHenselPairs() error {
	for h := s.henselPairs; h != nil; h = h.next {
		if h.executing || h.sourceExecuting || h.callDepth != 0 {
			return ErrUPolynomialCleanupPending
		}
	}
	for h := s.henselPairs; h != nil; h = h.next {
		if h.active {
			if e := s.cleanupHenselPairInvocation(&h.receipt, false); e != nil {
				return e
			}
			if e := s.cleanupHenselPairInvocation(&h.receipt, true); e != nil {
				return e
			}
		}
	}
	return nil
}

func (h *upolyHenselPairScope) enterCall() {
	if h.callDepth == 0 {
		m := h.service.manager
		h.rootPrevious = m.henselPairCaller
		m.henselPairCaller = h
		for _, pair := range []struct {
			manager  *UPolynomialManager
			previous **upolyHenselPairScope
		}{{h.check.brCore, &h.check.brPrevious}, {h.check.bCore, &h.check.bPrevious}} {
			if pair.manager != nil {
				*pair.previous = pair.manager.henselPairCaller
				pair.manager.henselPairCaller = h
			}
		}
		if h.rm != nil {
			h.rmPrevious = h.rm.henselPairCaller
			h.rm.henselPairCaller = h
		}
	}
	h.callDepth++
}
func (h *upolyHenselPairScope) attachRelated() {
	if h.callDepth != 0 && h.rm != nil {
		h.rmPrevious = h.rm.henselPairCaller
		h.rm.henselPairCaller = h
	}
}
func (h *upolyHenselPairScope) leaveCall() {
	h.callDepth--
	if h.callDepth == 0 {
		if h.check.brCore != nil {
			h.check.brCore.henselPairCaller = h.check.brPrevious
		}
		if h.check.bCore != nil {
			h.check.bCore.henselPairCaller = h.check.bPrevious
		}
		h.service.manager.henselPairCaller = h.rootPrevious
		if h.rm != nil {
			h.rm.henselPairCaller = h.rmPrevious
		}
	}
}
func (m *UPolynomialManager) henselDestructorAuthorized(r *upolyHenselPairInvocationReceipt, own bool) bool {
	if r == nil || r.service == nil || !r.service.matchesHenselPairInvocation(r) || !r.frame.ownsNumericManager(m) || m == r.service.manager || m.henselPairCaller != r.frame || m.callbackDepth != 0 || r.frame.callDepth == 0 {
		return false
	}
	h := r.frame
	if m == h.check.brCore || m == h.check.bCore {
		phase := uint8(2)
		if m == h.check.bCore {
			phase = 4
		}
		if own {
			return h.ownExecuting && h.check.ownExecuting
		}
		return (h.check.sourceExecuting && h.check.sourceRemaining == phase) || (h.ownExecuting && h.check.ownExecuting)
	}
	if own {
		return h.ownExecuting && h.ownRemaining == 1
	}
	return (h.sourceExecuting && h.sourceRemaining == 1) || (h.ownExecuting && h.ownRemaining == 1)
}

func (h *upolyHenselPairScope) cleanupRelatedSourceMembers() error {
	return cleanupHenselRelatedMembers(h.rm, &h.ownManagerStage, &h.ownManagerIndex)
}
func cleanupHenselRelatedMembers(m *UPolynomialManager, stage *uint8, index *uint32) error {
	if m == nil {
		return nil
	}
	derived := [5]*upolyVectorState{&m.db, &m.dbab1, &m.dbab2, &m.tr, &m.push}
	core := [16]*upolyVectorState{&m.basic, &m.div1, &m.div2, &m.exactDiv, &m.gcd1, &m.gcd2, &m.cra, &m.mgcd[0], &m.mgcd[1], &m.mgcd[2], &m.mgcd[3], &m.mgcd[4], &m.mgcd[5], &m.sqf1, &m.sqf2, &m.pw}
	for *stage < 5 {
		switch *stage {
		case 0, 2:
			states := derived[:]
			if *stage == 2 {
				states = core[:]
			}
			for _, v := range states {
				for *index < v.size {
					a := &m.values(v)[*index]
					if e := m.config.Integer.CleanupRawNumeral(a); e != nil {
						return e
					}
					*index++
				}
				v.size = 0
				*index = 0
			}
		case 1:
			for i := len(derived) - 1; i >= 0; i-- {
				m.freeBacking(derived[i])
			}
		case 3:
			for i := len(core) - 1; i >= 0; i-- {
				m.freeBacking(core[i])
			}
		case 4:
			for *index < 7 {
				if e := m.modular.CleanupRawMember(*index); e != nil {
					return e
				}
				*index++
			}
		}
		*stage++
		*index = 0
	}
	return nil
}
func (h *upolyHenselPairScope) cleanupNumeric(abort bool) error {
	z := h.service.manager.config.Integer
	for i, m := range h.numericManagers() {
		if m != nil {
			if e := m.CleanupPNormalizeInvocation(&h.normalize[i], abort); e != nil {
				return e
			}
		}
	}
	if e := z.CleanupAddMulInvocation(&h.addMul, abort); e != nil {
		return e
	}
	if e := z.CleanupDividesNumeralInvocation(&h.boolNumeral, abort); e != nil {
		return e
	}
	if e := z.CleanupRawInvocation(&h.divisionRaw, abort); e != nil {
		return e
	}
	if e := z.CleanupRawInvocation(&h.extendedRaw, abort); e != nil {
		return e
	}

	if abort {

		return h.numeric.cleanup(z)
	}
	return nil
}

func (h *upolyHenselPairScope) numericSourceCause() error {
	z := h.service.manager.config.Integer
	if h.check.sourceCause != nil {
		return h.check.sourceCause
	}
	if h.amSourceCause != nil {
		return h.amSourceCause
	}
	if e := z.AddMulInvocationSourceCause(&h.addMul); e != nil {
		return e
	}
	if e := z.DividesNumeralInvocationSourceCause(&h.boolNumeral); e != nil {
		return e
	}
	if e := z.RawInvocationSourceCause(&h.divisionRaw); e != nil {
		return e
	}
	if e := z.RawInvocationSourceCause(&h.extendedRaw); e != nil {
		return e
	}

	return nil
}
func (s *UPolynomialFactorizationService) finishHenselAmSource(r *upolyHenselPairInvocationReceipt, body error) error {
	if !s.matchesHenselPairHistory(r) {
		return ErrUPolynomialOwner
	}
	h := r.frame
	if h.amBodyCause == nil {
		h.amBodyCause = body
	}
	if h.amSourceCause == nil {
		h.amSourceCause = h.numericSourceCause()
	}
	if h.amSourceCause != nil || h.amEnded {
		return errors.Join(h.amBodyCause, h.amSourceCause)
	}
	_, e := upolynomialSourceNumeralDel(s.manager.config.Integer, &h.am)
	if e != nil {
		h.amSourceCause = e
		if h.sourceCause == nil {
			h.sourceCause = e
		}
		return errors.Join(h.amBodyCause, e)
	}
	h.amEnded = true
	return h.amBodyCause
}

// A source const numeral_vector reference follows its real vector header when
// it aliases an output. Each arithmetic call still receives its source SIZE and
// c_ptr at that call frontier; no numeral array or owning reference is cloned.
type upolyHenselInputBorrow struct {
	view  UPolynomialView
	state *upolyVectorState
}

func (h *upolyHenselPairScope) bindInputs(views [5]UPolynomialView, a, b *upolyVectorState) {
	m := h.service.manager
	for i, v := range views {
		h.inputs[i].view = v
		for _, out := range []*upolyVectorState{a, b} {
			if v.count == out.size && m.alias(v, out) {
				h.inputs[i].state = out
				break
			}
		}
	}
}
func (h *upolyHenselPairScope) borrowedInput(i int) UPolynomialView {
	r := &h.inputs[i]
	if r.state != nil {
		return h.service.manager.stateView(r.state)
	}
	return r.view
}
