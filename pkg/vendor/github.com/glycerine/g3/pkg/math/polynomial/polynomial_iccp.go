package polynomial

// Source: src/math/polynomial/polynomial.cpp:3432-3534;
// src/math/polynomial/polynomial.h:polynomial manager content overloads.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// One concrete lexical invocation, with native unsigned scratch and the ci
// reference constructed only after the integer content.
// The cold Go directory is an owning adapter, not a source allocation ticket.
type polynomialICCPScope struct {
	coefficient        polynomialCoefficientInvocationReceipt
	coefficientAppend  polynomialCheapAppendInvocationReceipt
	coefficientFactory polynomialFactoryInvocationReceipt
	neg                polynomialNegInvocationReceipt
	division           polynomialExactDivInvocationReceipt

	invocation              uint64
	childInvocation         uint64
	gcdInvocation           polynomialGCDInvocationReceipt
	contentInvocation       polynomialContentInvocationReceipt
	parent, child           *polynomialICCPScope
	filter, powers          []uint32
	ci                      polynomialReference
	active, ciConstructed   bool
	cursor                  uint8
	bodyCause, cleanupCause error
	header                  uint64
}

func (m *MultivariatePolynomialManager) iccpFrame() (*polynomialICCPScope, error) {
	if int(m.iccpDepth) < len(m.iccpFrames) {
		s := m.iccpFrames[m.iccpDepth]
		if s.active {
			return nil, ErrPolynomialPending
		}
		return s, nil
	}
	h := uint64(reflect.TypeOf(polynomialICCPScope{}).Size())
	if e := m.core.integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	s := &polynomialICCPScope{header: h}
	m.iccpFrames = append(m.iccpFrames, s)
	return s, nil
}
func (m *MultivariatePolynomialManager) iccpSourceEnd(s *polynomialICCPScope) error {
	if s.cleanupCause != nil {
		return s.cleanupCause
	}
	c := m.core
	if e := m.iccpConcreteChildSourceCause(s); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := c.contentInvocationSourceCause(&s.contentInvocation); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := m.gcdInvocationSourceCause(&s.gcdInvocation); e != nil {
		s.cleanupCause = e
		return e
	}
	if s.child != nil && s.child.invocation == s.childInvocation && s.child.active && s.child.cleanupCause != nil {
		s.cleanupCause = s.child.cleanupCause
		return s.cleanupCause
	}
	if s.cursor == 0 {
		if s.ciConstructed {
			if e := s.ci.closeValue(c, false); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.cursor = 1
	}
	if s.cursor == 1 {
		s.powers = nil
		s.cursor = 2
	}
	if s.cursor == 2 {
		s.filter = nil
		s.cursor = 3
	}
	if s.bodyCause == nil && !s.ci.assigning && !(s.child != nil && s.child.active && s.child.invocation == s.childInvocation) && !m.matchesGCDInvocation(&s.gcdInvocation) && !c.matchesContentInvocation(&s.contentInvocation) && !m.iccpConcreteChildrenPending(s) {
		s.retireICCPInvocation()
	}
	return nil
}
func (m *MultivariatePolynomialManager) flipLeadingSign(p *polynomialReference) error {
	return m.flipLeadingSignWithReceipt(p, nil)
}
func (m *MultivariatePolynomialManager) flipLeadingSignWithReceipt(p *polynomialReference, receipt *polynomialNegInvocationReceipt) error {
	c := m.core
	zero, e := c.IsZero(p.value)
	if e != nil || zero {
		return e
	}
	pos, e := c.gradedLexMaxPosition(p.value)
	if e != nil {
		return e
	}
	if e = polynomialFlipSourceAssert(func() (bool, error) { return pos != ^uint32(0), nil }); e != nil {
		return e
	}
	a, e := c.coefficientCore(p.value, pos)
	if e != nil {
		return e
	}
	negative, e := c.modular.IsNeg(&a)
	if e != nil || !negative {
		return e
	}
	q, e := c.negWithReceipt(p.value, receipt)
	if e != nil {
		return e
	}
	return p.assign(c, q)
}
func (m *MultivariatePolynomialManager) IntegerContentPrimitiveParts(p Polynomial, x uint32, i *unumeral.Int, c, pp *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	if i == nil || c == nil || pp == nil || c.owner != m.core || pp.owner != m.core {
		return ErrPolynomialOwner
	}
	if e := c.open(); e != nil {
		return e
	}
	if e := pp.open(); e != nil {
		return e
	}
	return m.iccpParts(p, x, i, &c.root, &pp.root)
}
func (m *MultivariatePolynomialManager) iccpParts(p Polynomial, x uint32, i *unumeral.Int, c, pp *polynomialReference) error {
	return m.iccpPartsWithReceipt(p, x, i, c, pp, nil)
}
func (m *MultivariatePolynomialManager) iccpPartsWithReceipt(p Polynomial, x uint32, i *unumeral.Int, c, pp *polynomialReference, receipt *polynomialICCPInvocationReceipt) (err error) {
	core := m.core
	if e := core.traceICCPEntry(p, x); e != nil {
		return e
	}
	zero, e := core.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		if e = core.modular.Reset(i); e != nil {
			return e
		}
		if e = c.assign(core, core.unit); e != nil {
			return e
		}
		return pp.assign(core, p)
	}
	constant, e := core.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		a, e := core.coefficientCore(p, 0)
		if e != nil {
			return e
		}
		e = m.iccpConstantSET(i, &a, receipt)
		if e != nil {
			return e
		}
		if e = c.assign(core, core.unit); e != nil {
			return e
		}
		return pp.assign(core, core.unit)
	}
	d, e := core.Degree(p, x)
	if e != nil {
		return e
	}
	if d == 0 {
		if e = core.integerContentForICCPShortcut(p, i, c, m, receipt); e != nil {
			return e
		}
		return pp.assign(core, core.unit)
	}
	s, e := m.iccpFrame()
	if e != nil {
		return e
	}
	if s.invocation == ^uint64(0) {
		return arena.ErrBounds
	}
	if e = core.pscEnter(); e != nil {
		return e
	}
	h := s.header
	*s = polynomialICCPScope{invocation: s.invocation + 1, active: true, header: h}
	if m.iccpDepth != 0 {
		s.parent = m.iccpFrames[m.iccpDepth-1]
		s.parent.child = s
		s.parent.childInvocation = s.invocation
	}
	if receipt != nil {
		*receipt = polynomialICCPInvocationReceipt{manager: m, frame: s, invocation: s.invocation}
	}
	m.iccpDepth++
	defer func() { m.iccpDepth--; core.pscLeave(); s.bodyCause = err; err = errors.Join(err, m.iccpSourceEnd(s)) }()
	s.filter = make([]uint32, d+1)
	s.powers = s.powers[:0]
	size, e := core.readWord(p, 8)
	if e != nil {
		return e
	}
	for j := uint32(0); j < size; j++ {
		term, e := core.monomialCore(p, j)
		if e != nil {
			return e
		}
		k, e := core.monomials.DegreeOf(term, x)
		if e != nil {
			return e
		}
		if e = core.traceICCPDegree(term, x, k); e != nil {
			return e
		}
		count := s.filter[k]
		if count == 0 {
			s.powers = append(s.powers, k)
		}
		sz, e := core.monomials.sizeCore(term)
		if e != nil {
			return e
		}
		expected := uint32(0)
		if k > 0 {
			expected = 1
		}
		// Source rereads the counter in ++ / += after the reached size read.
		count = s.filter[k]
		if sz == expected {
			count++
		} else {
			count += 2
		}
		s.filter[k] = count
	}
	if e = polynomialICCPSourceAssert(3501, "!iccp_powers.empty()", func() (bool, error) { return len(s.powers) != 0, nil }); e != nil {
		return e
	}
	number := uint32(len(s.powers))
	for j := uint32(0); j < number; j++ {
		k := s.powers[j]
		if e = polynomialICCPSourceAssert(3504, "iccp_filter[iccp_powers[j]] > 0", func() (bool, error) {
			return s.filter[k] > 0, nil
		}); e != nil {
			return e
		}
		k = s.powers[j]
		count := s.filter[k]
		if count == 1 {
			if e = core.integerContentWithReceipt(p, i, pp, &s.contentInvocation); e != nil {
				return e
			}
			return c.assign(core, core.unit)
		}
	}
	if e = core.integerContentWithReceipt(p, i, pp, &s.contentInvocation); e != nil {
		return e
	}
	if e = core.traceICCPContent(p, i, pp.value); e != nil {
		return e
	}
	s.ciConstructed = true
	k := s.powers[0]
	q, e := core.coeffCoreWithReceipts(pp.value, x, k, nil, &s.coefficient, &s.coefficientAppend, &s.coefficientFactory)
	if e != nil {
		return e
	}
	if e = c.assignWithFactoryReceipt(core, q, &s.coefficientFactory); e != nil {
		return e
	}
	for j := uint32(1); j < number; j++ {
		k := s.powers[j]
		q, e := core.coeffCoreWithReceipts(pp.value, x, k, nil, &s.coefficient, &s.coefficientAppend, &s.coefficientFactory)
		if e != nil {
			return e
		}
		if e = s.ci.assignWithFactoryReceipt(core, q, &s.coefficientFactory); e != nil {
			return e
		}
		if e = m.gcdIntoWithReceipt(c.value, s.ci.value, c, &s.gcdInvocation); e != nil {
			return e
		}
		constant, e := core.IsConstant(c.value)
		if e != nil {
			return e
		}
		if constant {
			return c.assign(core, core.unit)
		}
	}
	if e = polynomialICCPSourceAssert(3525, "!is_const(c)", func() (bool, error) {
		constant, x := core.IsConstant(c.value)
		return !constant, x
	}); e != nil {
		return e
	}
	if e = m.flipLeadingSignWithReceipt(c, &s.neg); e != nil {
		return e
	}
	if e = core.traceICCPDivisor(pp.value, c.value); e != nil {
		return e
	}
	q, e = core.exactDivWithReceipt(pp.value, c.value, &s.division)
	if e != nil {
		return e
	}
	return pp.assign(core, q)
}

// CleanupContentPrimitiveParts is the separate owning continuation. Completed
// younger source objects are acknowledged before unjoined reference INCs.
func (m *MultivariatePolynomialManager) CleanupContentPrimitiveParts() error {
	if m == nil || m.self != m || m.core == nil {
		return ErrPolynomialOwner
	}
	if m.iccpDepth != 0 {
		return ErrPolynomialPending
	}
	if e := m.cleanupICCPConstantInvocation(&m.iccpConstant.directReceipt, false); e != nil {
		return e
	}
	if e := m.core.cleanupContentInvocation(&m.iccpShortcutContent, false); e != nil {
		return e
	}
	if e := m.cleanupICCPScopes(m.iccpFrames, false); e != nil {
		return e
	}
	if e := m.cleanupICCPScopes(m.iccpFrames, true); e != nil {
		return e
	}
	if e := m.cleanupICCPConstantInvocation(&m.iccpConstant.directReceipt, true); e != nil {
		return e
	}
	return m.core.cleanupContentInvocation(&m.iccpShortcutContent, true)
}
func (m *MultivariatePolynomialManager) cleanupICCPScopes(scopes []*polynomialICCPScope, abort bool) error {
	c := m.core
	if !abort {
		for j := len(scopes) - 1; j >= 0; j-- {
			s := scopes[j]
			if !s.active {
				continue
			}
			if e := m.cleanupICCPConcreteChildren(s, false); e != nil {
				return e
			}
			if e := m.cleanupGCDInvocation(&s.gcdInvocation, false); e != nil {
				return e
			}
			if c.matchesContentInvocation(&s.contentInvocation) {
				if e := c.cleanupContentInvocation(&s.contentInvocation, false); e != nil {
					return e
				}
			}
			if s.ciConstructed {
				if e := s.ci.closeValue(c, true); e != nil {
					return e
				}
			}
			s.powers = nil
			s.filter = nil
		}
		return nil
	}
	for j := len(scopes) - 1; j >= 0; j-- {
		s := scopes[j]
		if !s.active {
			continue
		}
		if e := m.cleanupICCPConcreteChildren(s, true); e != nil {
			return e
		}
		if e := m.cleanupGCDInvocation(&s.gcdInvocation, true); e != nil {
			return e
		}
		if c.matchesContentInvocation(&s.contentInvocation) {
			if e := c.cleanupContentInvocation(&s.contentInvocation, true); e != nil {
				return e
			}
		}
		if e := s.ci.abort(c); e != nil {
			return e
		}
		s.powers = nil
		s.filter = nil
	}
	for _, s := range scopes {
		if s.active {
			s.retireICCPInvocation()
		}
	}
	return nil
}
func (m *MultivariatePolynomialManager) iccpPending() bool {
	if m.iccpConstant.active {
		return true
	}
	if m.core.matchesContentInvocation(&m.iccpShortcutContent) {
		return true
	}
	for _, s := range m.iccpFrames {
		if s.active {
			return true
		}
	}
	return false
}
