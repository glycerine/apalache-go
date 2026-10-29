package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:703-801,845-933.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// These are the reached caller declarations, embedded in the actual charged
// factor_square_free owner. The direct pair receipt is acquired by the child
// at its real begin; no cached service scope supplies caller authority.

type upolyHenselCallerScope struct {
	individual                                              [1]upolyVectorState
	individualSource, individualOwn                         uint8
	checkVectors                                            [4]upolyVectorState // mult_zp,f_zp then mult_zpe,f_zpe
	checkSource, checkOwn                                   uint8
	checkPower                                              unumeral.Int
	checkPowerConstructed, checkPowerEnded, checkPowerOwned bool
	checkPowerInvocation                                    unumeral.IntegerPowerInvocationReceipt

	quadraticCheck                                                     [3]upolyVectorState // tmp1,tmp2 before MUL; one after both MULs
	quadraticCheckConstructed, quadraticCheckSource, quadraticCheckOwn uint8
	initialLC, finalLCInv, transport                                   unumeral.Int
	initialLCConstructed, initialLCEnded, initialLCOwned               bool
	finalLCInvConstructed, finalLCInvEnded, finalLCInvOwned            bool
	transportBound                                                     bool
	callDepth                                                          uint32
	callOwners                                                         [4]*UPolynomialManager
	callerPrevious                                                     [4]*upolyHenselCallerScope
	callerPreviousInvocation                                           [4]uint64
	extended                                                           [6]upolyVectorState // V1,V3,Q,R,T,V1Q, actual ext_gcd declarations
	extendedDebug                                                      [2]upolyVectorState // BV, expectedD at reached DEBUG declaration
	extendedScoped                                                     [2]unumeral.Int     // lc_inv, lc at final reached lexical block
	monicRaw                                                           [2]unumeral.Int     // lc, lc_inv in header mk_monic overload; no automatic DT
	extendedConstructed, extendedSource, extendedOwn                   uint8
	extendedDebugConstructed, extendedDebugSource, extendedDebugOwn    uint8
	extendedScopedConstructed, extendedScopedSource, extendedScopedOwn uint8
	extendedRawConstructed, extendedRawEndEntered, extendedRawEnded    bool
	extendedRawSource, extendedRawOwn                                  uint8
	extendedActive                                                     bool

	am                              unumeral.Int
	amConstructed, amEnded, amOwned bool
	amBodyCause, amSourceCause      error
	invocation                      uint64
	executing                       bool
	prime, lift                     *UPolynomialManager
	numeric                         polynomialNumericRoots
	normalize                       [4]unumeral.IntegerNormalizeInvocationReceipt
	addMul                          unumeral.IntegerAddMulInvocationReceipt
	divisionRaw, extendedRaw        unumeral.IntegerRawInvocationReceipt
	boolNumeral                     unumeral.IntegerDividesNumeralInvocationReceipt

	owner                                                              *upolyFactorSquareFreeScope
	active                                                             bool
	pair                                                               upolyHenselPairInvocationReceipt
	outer                                                              [4]upolyVectorState // A, B, C, parts
	quadratic                                                          [5]upolyVectorState // U, V, D, A_lifted, B_lifted
	iteration                                                          [6]upolyVectorState // tmp1, g, S, T, t, tmp2
	related                                                            *UPolynomialManager
	outerConstructed, quadraticConstructed, iterationConstructed       uint8
	outerSource, quadraticSource, iterationSource                      uint8
	outerOwn, quadraticOwn, iterationOwn                               uint8
	outerSourceStarted, quadraticSourceStarted, iterationSourceStarted bool
	outerOwnStarted, quadraticOwnStarted, iterationOwnStarted          bool
	ownIndex                                                           uint32
	sourceExecuting, ownExecuting                                      bool
	sourceIndex                                                        uint32
	bodyCause                                                          error
	sourceCause                                                        error
	ownCause                                                           error
	ownRelatedStage                                                    uint8
	ownRelatedIndex                                                    uint32
}

func (s *UPolynomialFactorizationService) beginHenselCaller(owner *upolyFactorSquareFreeScope) error {
	if s == nil || s.self != s || owner == nil || owner.self != owner || owner.service != s || s.scope != owner || owner.manager != s.manager || s.manager == nil || s.manager.self != s.manager {
		return ErrUPolynomialOwner
	}
	if owner.hensel.active || owner.hensel.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	owner.hensel = upolyHenselCallerScope{owner: owner, active: true, executing: true, invocation: owner.hensel.invocation + 1}
	return nil
}
func (c *upolyHenselCallerScope) matches() bool {
	return c != nil && c.owner != nil && c.owner.self == c.owner && c.owner.service != nil && c.owner.service.self == c.owner.service && c.owner.service.scope == c.owner && c.owner.manager == c.owner.service.manager && c.owner.manager != nil && c.owner.manager.self == c.owner.manager && c == &c.owner.hensel && c.active
}
func (c *upolyHenselCallerScope) childCause() error {
	if !c.matches() {
		return ErrUPolynomialOwner
	}
	if e := c.owner.traceSourceBlocker(); e != nil {
		return e
	}
	if e := c.owner.service.henselPairInvocationSourceCause(&c.pair); e != nil {
		return e
	}
	return c.numericSourceCause()
}
func (c *upolyHenselCallerScope) finishVectors(v []upolyVectorState, count *uint8) error {
	m := c.owner.manager
	for *count != 0 {
		if e := upolynomialScopedVectorSourceDel(m, &v[*count-1], &c.sourceIndex); e != nil {
			c.sourceCause = e
			return e
		}
		*count--
		c.sourceIndex = 0
	}
	return nil
}
func (c *upolyHenselCallerScope) finishIteration() error {
	c.enterCall()
	defer c.leaveCall()
	previous := c.sourceExecuting
	c.sourceExecuting = true
	defer func() { c.sourceExecuting = previous }()
	if e := c.finishQuadraticCheckSource(); e != nil {
		return e
	}
	if c.extendedActive {
		if e := c.finishExtendedSource(); e != nil {
			return e
		}
	}
	if c.sourceCause != nil {
		return c.sourceCause
	}
	if e := c.childCause(); e != nil {
		c.sourceCause = e
		return e
	}
	if !c.iterationSourceStarted {
		c.iterationSourceStarted = true
		c.iterationSource = c.iterationConstructed
	}
	return c.finishVectors(c.iteration[:], &c.iterationSource)
}
func (c *upolyHenselCallerScope) finishQuadratic() error {
	c.enterCall()
	defer c.leaveCall()
	previous := c.sourceExecuting
	c.sourceExecuting = true
	defer func() { c.sourceExecuting = previous }()
	if e := c.finishIteration(); e != nil {
		return e
	}
	if !c.quadraticSourceStarted {
		c.quadraticSourceStarted = true
		c.quadraticSource = c.quadraticConstructed
	}
	if e := c.finishVectors(c.quadratic[:], &c.quadraticSource); e != nil {
		return e
	}
	if c.related != nil {
		if e := c.related.sourceFinalizeWithHenselOwners(nil, c); e != nil {
			c.sourceCause = e
			return e
		}

	}
	return nil
}
func (c *upolyHenselCallerScope) finish() error {
	if !c.matches() {
		return ErrUPolynomialOwner
	}
	if c.executing || c.sourceExecuting || c.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	c.enterCall()
	defer c.leaveCall()
	previous := c.sourceExecuting
	c.sourceExecuting = true
	defer func() { c.sourceExecuting = previous }()
	if e := c.finishConditionalSource(); e != nil {
		return e
	}
	if e := c.finishCallerNumeral(false); e != nil {
		return e
	}
	if e := c.finishCallerNumeral(true); e != nil {
		return e
	}
	if e := c.finishQuadratic(); e != nil {
		return e
	}
	if !c.outerSourceStarted {
		c.outerSourceStarted = true
		c.outerSource = c.outerConstructed
	}
	if e := c.finishVectors(c.outer[:], &c.outerSource); e != nil {
		return e
	}

	return nil
}
func (c *upolyHenselCallerScope) ownVectors(v []upolyVectorState, count *uint8) error {
	m := c.owner.manager
	for *count != 0 {
		p := &v[*count-1]
		for c.ownIndex < p.size {
			if e := m.config.Integer.CleanupRawNumeral(&m.values(p)[c.ownIndex]); e != nil {
				return e
			}
			c.ownIndex++
		}
		p.size = 0
		m.freeBacking(p)
		*count--
		c.ownIndex = 0
	}
	return nil
}
func (c *upolyHenselCallerScope) own(abort bool) (cause error) {
	if !c.active {
		return nil
	}
	if !c.matches() {
		return ErrUPolynomialOwner
	}
	if c.executing || c.sourceExecuting || c.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	defer func() {
		if cause != nil && c.ownCause == nil {
			c.ownCause = cause
		}
	}()
	c.enterCall()
	defer c.leaveCall()
	s := c.owner.service
	c.ownExecuting = true
	defer func() { c.ownExecuting = false }()
	if e := c.ownConditional(); e != nil {
		return e
	}
	if e := c.cleanupNumeric(false); e != nil {
		return e
	}
	for _, n := range []struct {
		value              *unumeral.Int
		constructed, ended bool
		owned              *bool
	}{{&c.finalLCInv, c.finalLCInvConstructed, c.finalLCInvEnded, &c.finalLCInvOwned}, {&c.initialLC, c.initialLCConstructed, c.initialLCEnded, &c.initialLCOwned}} {
		if n.constructed && !n.ended && !*n.owned {
			if e := c.owner.manager.config.Integer.CleanupRawNumeral(n.value); e != nil {
				return e
			}
			*n.owned = true
		}
	}
	if c.amConstructed && !c.amEnded && !c.amOwned {
		if e := c.owner.manager.config.Integer.CleanupRawNumeral(&c.am); e != nil {
			return e
		}
		c.amOwned = true
	}
	if !c.iterationOwnStarted {
		c.iterationOwnStarted = true
		c.iterationOwn = c.iterationConstructed
	}
	if !c.quadraticOwnStarted {
		c.quadraticOwnStarted = true
		c.quadraticOwn = c.quadraticConstructed
	}
	if !c.outerOwnStarted {
		c.outerOwnStarted = true
		c.outerOwn = c.outerConstructed
	}
	if e := s.cleanupHenselPairInvocation(&c.pair, false); e != nil {
		return e
	}
	if e := c.ownExtended(false); e != nil {
		return e
	}
	if e := c.ownVectors(c.quadraticCheck[:], &c.quadraticCheckOwn); e != nil {
		return e
	}
	if e := c.ownVectors(c.iteration[:], &c.iterationOwn); e != nil {
		return e
	}
	if e := c.ownVectors(c.quadratic[:], &c.quadraticOwn); e != nil {
		return e
	}
	if c.related != nil {
		if e := cleanupHenselRelatedMembers(c.related, &c.ownRelatedStage, &c.ownRelatedIndex); e != nil {
			return e
		}
	}
	if e := c.ownVectors(c.outer[:], &c.outerOwn); e != nil {
		return e
	}
	if abort {
		if e := c.owner.manager.config.Integer.CleanupPowerInvocation(&c.checkPowerInvocation, true); e != nil {
			return e
		}
		if c.transportBound {
			if e := c.owner.manager.config.Integer.CleanupRawNumeral(&c.transport); e != nil {
				return e
			}
			c.transportBound = false
		}
		if e := c.ownExtended(true); e != nil {
			return e
		}
		if e := c.cleanupNumeric(true); e != nil {
			return e
		}
		if e := s.cleanupHenselPairInvocation(&c.pair, true); e != nil {
			return e
		}
		if c.related != nil {
			if e := c.related.closeWithHenselOwners(nil, c); e != nil {
				return e
			}
			c.detachRelated()
			c.related = nil
		}
		c.active = false
	}
	return nil
}
func joinHenselCaller(body, source error) error { return errors.Join(body, source) }

func (h *upolyHenselCallerScope) cleanupNumeric(abort bool) error {
	z := h.owner.manager.config.Integer
	if e := z.CleanupPowerInvocation(&h.checkPowerInvocation, abort); e != nil {
		return e
	}
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

func (h *upolyHenselCallerScope) numericSourceCause() error {
	z := h.owner.manager.config.Integer
	if e := z.PowerInvocationSourceCause(&h.checkPowerInvocation); e != nil {
		return e
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

func (c *upolyHenselCallerScope) finishAmSource(body error) error {
	if c.amBodyCause == nil {
		c.amBodyCause = body
	}
	if c.amSourceCause == nil {
		c.amSourceCause = c.numericSourceCause()
	}
	if c.amSourceCause != nil {
		return c.amSourceCause
	}
	if c.amEnded {
		return c.amBodyCause
	}
	_, e := upolynomialSourceNumeralDel(c.owner.manager.config.Integer, &c.am)
	if e != nil {
		c.amSourceCause = e
		if c.sourceCause == nil {
			c.sourceCause = e
		}
		return e
	}
	c.amEnded = true
	return c.amBodyCause
}
func henselNumericAddMulChild(r *upolyHenselPairInvocationReceipt, c *upolyHenselCallerScope) *unumeral.IntegerAddMulInvocationReceipt {
	if c != nil {
		return &c.addMul
	}
	return henselAddMulChild(r)
}

func (c *upolyHenselCallerScope) finishCallerNumeral(initial bool) error {
	n, constructed, ended := &c.finalLCInv, c.finalLCInvConstructed, &c.finalLCInvEnded
	if initial {
		n, constructed, ended = &c.initialLC, c.initialLCConstructed, &c.initialLCEnded
	}
	if !constructed || *ended {
		return nil
	}
	if c.sourceCause != nil {
		return c.sourceCause
	}
	if e := c.owner.traceSourceBlocker(); e != nil {
		c.sourceCause = e
		return e
	}
	if e := c.numericSourceCause(); e != nil {
		c.sourceCause = e
		return e
	}
	if _, e := upolynomialSourceNumeralDel(c.owner.manager.config.Integer, n); e != nil {
		c.sourceCause = e
		return e
	}
	*ended = true
	return nil
}

func (c *upolyHenselCallerScope) finishQuadraticCheckSource() error {
	if c.sourceCause != nil {
		return c.sourceCause
	}
	if e := c.owner.traceSourceBlocker(); e != nil {
		c.sourceCause = e
		return e
	}
	if e := c.numericSourceCause(); e != nil {
		c.sourceCause = e
		return e
	}
	return c.finishVectors(c.quadraticCheck[:], &c.quadraticCheckSource)
}

// A successful completed callee may refund its Go adapter storage before the
// next source call constructs a new related manager. This never rescues an
// exception's unjoined RAW: the body/source completion frontier is mandatory.
func (c *upolyHenselCallerScope) completeQuadraticStorage() error {
	if !c.matches() || c.sourceCause != nil || c.quadraticSource != 0 || c.iterationSource != 0 || c.extendedSource != 0 {
		return ErrUPolynomialCleanupPending
	}
	if c.related == nil || !c.related.sourceFinalized {
		return ErrUPolynomialCleanupPending
	}
	if polynomialNegNumericPending(&c.numeric) || c.transportBound {
		return ErrUPolynomialCleanupPending
	}
	s := c.owner.service
	if s.matchesHenselPairInvocation(&c.pair) {
		h := c.pair.frame
		if !h.sourceEnded || h.bodyCause != nil || h.sourceCause != nil {
			return ErrUPolynomialCleanupPending
		}
	}
	c.ownExecuting = true
	defer func() { c.ownExecuting = false }()
	if e := c.cleanupNumeric(false); e != nil {
		return e
	}
	if e := s.cleanupHenselPairInvocation(&c.pair, false); e != nil {
		return e
	}
	if e := s.cleanupHenselPairInvocation(&c.pair, true); e != nil {
		return e
	}
	if e := c.cleanupNumeric(true); e != nil {
		return e
	}
	if e := c.related.closeWithHenselOwners(nil, c); e != nil {
		return e
	}
	c.detachRelated()
	c.related = nil
	c.quadraticOwnStarted = false
	c.iterationOwnStarted = false
	c.quadraticOwn = 0
	c.iterationOwn = 0
	return nil
}
