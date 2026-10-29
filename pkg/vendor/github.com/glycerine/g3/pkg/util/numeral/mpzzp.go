package numeral

// Source: src/util/mpzzp.h.
// Concrete source util/mpzzp.h manager; borrows one actual IntegerManager.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrModularOwner = errors.New("mpnum: invalid modular integer owner")

var ErrModularPrimeUninitialized = errors.New("mpnum: source modular constructor left prime flag uninitialized")
var ErrModularAssertion = errors.New("mpnum: source modular assertion failed")
var ErrModularPowerPending = errors.New("mpnum: modular power raw integer cleanup pending")

// IntegerModularManager owns seven source numerals and borrows its integer
// manager. It is caller-confined; Int operands obey the borrowed manager's
// implicit ownership contract. Explicit Close replaces the source destructor.
type IntegerModularManager struct {
	callerBindings                                     *NumericCallerBinding
	CallerExecuting                                    func() bool
	Normalization                                      integerPNormalizeScope
	self                                               *IntegerModularManager
	manager                                            *IntegerManager
	trace                                              *diagnostic.TraceTags
	z, prime, primeKnown, closed                       bool
	P, Lower, Upper, InvTmp1, InvTmp2, InvTmp3, DivTmp Int
	PowerTemporary                                     Int
	PowerPending                                       bool
	closing                                            bool
	closeCursor                                        uint8
	Header                                             uint64
}

func NewIntegerModularManager(z *IntegerManager) (*IntegerModularManager, error) {
	if z == nil {
		return nil, arena.ErrConfig
	}
	if z.closed {
		return nil, arena.ErrClosed
	}
	n := uint64(reflect.TypeOf(IntegerModularManager{}).Size())
	if e := z.BudgetField.Reserve(n); e != nil {
		return nil, e
	}
	m := &IntegerModularManager{manager: z, z: true, Header: n}
	m.self = m
	return m, nil
}

// Source ignores prime and never initializes its bool. Only actual reads of
// that indeterminate field are rejected; normalization does not read it.
func NewIntegerModularManagerWithModulus(z *IntegerManager, p *Int, prime bool) (*IntegerModularManager, error) {
	m, e := NewIntegerModularManager(z)
	if e != nil {
		return nil, e
	}
	m.z = false
	if e = z.Copy(&m.P, p); e == nil {
		e = m.setupP()
	}
	if e != nil {
		m.Close()
		return nil, e
	}
	return m, nil
}
func NewIntegerModularManagerWithUint64(z *IntegerManager, p uint64, prime bool) (*IntegerModularManager, error) {
	m, e := NewIntegerModularManager(z)
	if e != nil {
		return nil, e
	}
	m.z = false
	if e = z.SetUint64(&m.P, p); e == nil {
		e = m.setupP()
	}
	if e != nil {
		m.Close()
		return nil, e
	}
	return m, nil
}
func (m *IntegerModularManager) check() error {
	if m == nil || m.self != m || m.manager == nil {
		return ErrModularOwner
	}
	if m.closed || m.manager.closed {
		return arena.ErrClosed
	}
	if m.closing {
		return ErrNumericPending
	}
	return nil
}
func (m *IntegerModularManager) Manager() (*IntegerManager, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	return m.manager, nil
}
func (m *IntegerModularManager) Precise() bool { return true }
func (m *IntegerModularManager) OwnerBytes() uint64 {
	if m == nil || m.self != m || m.closed {
		return 0
	}
	return m.Header
}
func (m *IntegerModularManager) Close() error {
	if m.numericCallerExecuting() {
		return ErrNumericPending
	}
	if m != nil && m.self == m && m.Normalization.active {
		return ErrNumericPending
	}
	if m == nil || m.self != m || m.manager == nil {
		return ErrModularOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if !m.manager.closed {
		if !m.closing {
			if e := m.CleanupPowerTemporary(); e != nil {
				return e
			}
			m.closing = true
		}
		values := [...]*Int{&m.P, &m.Lower, &m.Upper, &m.InvTmp1, &m.InvTmp2, &m.InvTmp3, &m.DivTmp}
		for m.closeCursor < uint8(len(values)) {
			if e := m.manager.delWithModularOwner(values[m.closeCursor], m); e != nil {
				return e
			}
			m.closeCursor++
		}
	}
	m.manager.BudgetField.Release(m.Header)
	m.closed = true
	return nil
}
func (m *IntegerModularManager) Field() (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if m.z {
		return false, nil
	}
	if !m.primeKnown {
		return false, ErrModularPrimeUninitialized
	}
	return m.prime, nil
}
func (m *IntegerModularManager) Finite() (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	return !m.z, nil
}
func (m *IntegerModularManager) Modular() (bool, error) { return m.Finite() }
func (m *IntegerModularManager) Modulus(out *Int) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.manager.Copy(out, &m.P)
}
func (m *IntegerModularManager) setupP() error {
	return m.setupPWithAllocationContext(integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) setupPWithAllocationContext(context integerAllocationInvocationContext) error {
	z := m.manager
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			pos, e := z.IsPos(&m.P)
			if e != nil {
				return e
			}
			valid := pos
			if pos {
				one, e := z.IsOne(&m.P)
				if e != nil {
					return e
				}
				valid = !one
			}
			if !valid {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpzzp.h", 47, "m().is_pos(m_p) && !m().is_one(m_p)"); e != nil {
					return e
				}
			}
		}
	}
	even, e := z.IsEven(&m.P)
	if e != nil {
		return e
	}
	two := NewInt(2)
	if e = z.divWithAllocationContext(&m.P, &two, &m.Upper, context); e != nil {
		return e
	}
	if e = z.copyFromOwnerWithAllocationContext(&m.Lower, z, &m.Upper, nil, context); e != nil {
		return e
	}
	if e = z.negWithAllocationContext(&m.Lower, context); e != nil {
		return e
	}
	if even {
		one := NewInt(1)
		if e := z.addSubWithAllocationContext(&m.Lower, &one, &m.Lower, false, context); e != nil {
			return e
		}
	}
	return m.traceSetupP()
}
func (m *IntegerModularManager) SetZ() error {
	return m.setZWithAllocationContext(integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) setZWithAllocationContext(context integerAllocationInvocationContext) error {
	if m != nil && m.self == m && callerBindingsExecuting(m.callerBindings) && !m.acceptedModeContext(context) {
		return ErrNumericPending
	}
	if e := m.check(); e != nil {
		return e
	}
	m.z = true
	return nil
}
func (m *IntegerModularManager) SetZP(p *Int) error {
	return m.setZPWithAllocationContext(p, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) setZPWithAllocationContext(p *Int, context integerAllocationInvocationContext) error {
	if m != nil && m.self == m && callerBindingsExecuting(m.callerBindings) && !m.acceptedModeContext(context) {
		return ErrNumericPending
	}
	if e := m.check(); e != nil {
		return e
	}
	m.z = false
	m.prime = true
	m.primeKnown = true
	if e := m.manager.copyFromOwnerWithAllocationContext(&m.P, m.manager, p, nil, context); e != nil {
		return e
	}
	return m.setupPWithAllocationContext(context)
}
func (m *IntegerModularManager) SetZPUint64(p uint64) error {
	if m != nil && m.self == m && callerBindingsExecuting(m.callerBindings) {
		return ErrNumericPending
	}
	if e := m.check(); e != nil {
		return e
	}
	m.z = false
	m.prime = true
	m.primeKnown = true
	if e := m.manager.SetUint64(&m.P, p); e != nil {
		return e
	}
	return m.setupP()
}
func (m *IntegerModularManager) SetPSquare() error {
	return m.setPSquareWithAllocationContext(integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) SetZPSwap(p *Int) error {
	if m != nil && m.self == m && callerBindingsExecuting(m.callerBindings) {
		return ErrNumericPending
	}
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertFinite(126); e != nil {
		return e
	}
	if e := m.manager.Swap(&m.P, p); e != nil {
		return e
	}
	return m.setupP()
}
func (m *IntegerModularManager) IsPNormalized(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if m.z {
		return true, nil
	}
	ge, e := m.manager.GreaterEqual(a, &m.Lower)
	if e != nil || !ge {
		return ge, e
	}
	return m.manager.LessEqual(a, &m.Upper)
}
func (m *IntegerModularManager) AssertNormalized(a *Int) error {
	if !modularSourceDebug {
		return nil
	}
	b, e := m.IsPNormalized(a)
	if e != nil {
		return e
	}
	if !b {
		return ErrModularAssertion
	}
	return nil
}
func (m *IntegerModularManager) PNormalize(a *Int) error { return m.PNormalizeWithReceipt(a, nil) }
func (m *IntegerModularManager) PNormalizeWithReceipt(a *Int, receipt *integerPNormalizeInvocationReceipt) error {
	return m.PNormalizeWithFactorHistory(a, receipt, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) PNormalizeWithFactorHistory(a *Int, receipt *integerPNormalizeInvocationReceipt, history integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	var scope *integerPNormalizeScope
	if receipt != nil && !m.z {
		// These are the reached Rem operand/divisor guards, before invocation.
		if e := m.manager.check(a); e != nil {
			return e
		}
		if e := m.manager.check(&m.P); e != nil {
			return e
		}
		if m.P.Val == 0 {
			return ErrDivideByZero
		}
		scope = &m.Normalization
		if scope.active || scope.invocation == ^uint64(0) {
			return ErrNumericPending
		}
		*scope = integerPNormalizeScope{factorHistoryContext: history, owner: m, invocation: scope.invocation + 1, active: true, executing: true, Stage: 1}
		*receipt = integerPNormalizeInvocationReceipt{m, scope, scope.invocation}
		defer func() {
			scope.executing = false
			if !scope.pending() {
				scope.active = false
			}
		}()
	}
	if !m.z {
		if e := m.sourceAssertFinite(59); e != nil {
			return e
		}
		// This execution flag only protects the actual numerical owner from
		// returning-transport destruction. Allocation authority stays explicit.
		previousExecuting := m.manager.NormalizationExecuting
		m.manager.NormalizationExecuting = true
		defer func() { m.manager.NormalizationExecuting = previousExecuting }()
		if e := m.manager.remWithPNReceipt(a, &m.P, a, scope); e != nil {
			return e
		}
		if scope != nil {
			scope.Stage = 2
		}
		gt, e := m.manager.Greater(a, &m.Upper)
		if e != nil {
			return e
		}
		if gt {
			if e = m.manager.addSubWithPNReceipt(a, &m.P, a, true, scope); e != nil {
				return e
			}
		} else {
			lt, e := m.manager.Less(a, &m.Lower)
			if e != nil {
				return e
			}
			if lt {
				if e = m.manager.addSubWithPNReceipt(a, &m.P, a, false, scope); e != nil {
					return e
				}
			}
		}
		if e := m.sourceAssertNormalizedNamed(a, 68, "is_p_normalized(x)"); e != nil {
			return e
		} // core diagnostic
	}
	return m.sourceAssertNormalizedNamed(a, 114, "is_p_normalized(x)")
}
func (m *IntegerModularManager) Inv(a *Int) error {
	return m.invWithAllocationContext(a, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) invWithAllocationContext(a *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if m.z {
		return diagnostic.BorrowProcessDebugContext().UnreachableSource("src/util/mpzzp.h", 169)
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			zero, e := m.IsZero(a)
			if e != nil {
				return e
			}
			if zero {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpzzp.h", 172, "!is_zero(a)"); e != nil {
					return e
				}
			}
		}
	}
	if e := m.traceInvStart(a); e != nil {
		return e
	}
	if e := m.normalizeWithSourceContext(a, context); e != nil {
		return e
	}
	if e := m.traceInvNormalized(a); e != nil {
		return e
	}
	if e := m.manager.extendedGCDWithAllocationContext(a, &m.P, &m.InvTmp1, &m.InvTmp2, &m.InvTmp3, context); e != nil {
		return e
	}
	if e := m.traceInvGCD(); e != nil {
		return e
	}
	if e := m.normalizeWithSourceContext(&m.InvTmp1, context); e != nil {
		return e
	}
	if e := m.manager.Swap(a, &m.InvTmp1); e != nil {
		return e
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			one, e := m.manager.IsOne(&m.InvTmp3)
			if e != nil {
				return e
			}
			if !one {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpzzp.h", 183, "m().is_one(m_inv_tmp3)"); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func (m *IntegerModularManager) Div(a, b, c *Int) error {
	return m.divWithAllocationContext(a, b, c, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) divWithAllocationContext(a, b, c *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if m.z {
		return m.manager.divWithAllocationContext(a, b, c, context)
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			f, e := m.Field()
			if e != nil {
				return e
			}
			if !f {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/mpzzp.h", 200, "m_p_prime"); e != nil {
					return e
				}
			}
		}
	}
	if e := m.sourceAssertNormalized(a, 201); e != nil {
		return e
	}
	if e := m.manager.copyFromOwnerWithAllocationContext(&m.DivTmp, m.manager, b, nil, context); e != nil {
		return e
	}
	if e := m.invWithAllocationContext(&m.DivTmp, context); e != nil {
		return e
	}
	if e := m.mulWithSourceContext(a, &m.DivTmp, c, context); e != nil {
		return e
	}
	return m.sourceAssertNormalized(c, 205)
}
func (m *IntegerModularManager) Divides(a, b *Int) (bool, error) {
	return m.dividesWithAllocationContext(a, b, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) GCD(a, b, c *Int) error {
	return m.gcdWithSourceContext(a, b, c, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) gcdWithSourceContext(a, b, c *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 212); e != nil {
		return e
	}
	if e := m.manager.gcdWithAllocationContext(a, b, c, context); e != nil {
		return e
	}
	return m.sourceAssertNormalizedNamed(c, 214, "is_p_normalized(c)")
}
func (m *IntegerModularManager) GCDMany(as []Int, g *Int) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.GCDMany(as, g); e != nil {
		return e
	}
	return m.sourceAssertNormalizedNamed(g, 219, "is_p_normalized(g)")
}
func (m *IntegerModularManager) ExtendedGCD(r1, r2, a, b, g *Int) error {
	return m.extendedGCDWithSourceContext(r1, r2, a, b, g, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) extendedGCDWithSourceContext(r1, r2, a, b, g *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalizedPairNamed(r1, r2, 223, "is_p_normalized(r1) && is_p_normalized(r2)"); e != nil {
		return e
	}
	if e := m.manager.extendedGCDWithAllocationContext(r1, r2, a, b, g, context); e != nil {
		return e
	}
	if e := m.normalizeWithSourceContext(a, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(b, context)
}
func (m *IntegerModularManager) Power(a *Int, k uint32, b *Int) error {
	return m.powerWithSourceContext(a, k, b, integerAllocationInvocationContext{})
}

// Source: src/util/mpzzp.h:242-254. The local power is RAW, with end-only DEL.
func (m *IntegerModularManager) powerWithSourceContext(a *Int, k uint32, b *Int, context integerAllocationInvocationContext) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	if m.PowerPending {
		return ErrModularPowerPending
	}
	if e := m.sourceAssertNormalized(a, 243); e != nil {
		return e
	}
	var localPower Int
	power := &localPower
	var finish func() error
	var borrowed NumericPowerInvocation
	var policy NumericCallerPowerPolicy
	if context.Caller != nil {
		policy, _ = context.Caller.Policy.(NumericCallerPowerPolicy)
	}
	if policy != nil && policy.NumericPowerEnabled() {
		var e error
		borrowed, e = policy.NumericPower(m)
		if e != nil {
			return e
		}
		if borrowed == nil || borrowed.NumericPowerValue() == nil {
			return ErrNumericCallerOwner
		}
		power = borrowed.NumericPowerValue()
		defer func() { borrowed.NumericPowerEnd(cause) }()
	} else if context.Caller != nil && context.Caller.Power != nil {
		var end func(error)
		var e error
		power, finish, end, e = context.Caller.Power(m)
		if e != nil {
			return e
		}
		if power == nil || finish == nil || end == nil {
			return ErrNumericCallerOwner
		}
		defer func() { end(cause) }()
	} else {
		m.PowerTemporary = Int{}
		m.PowerPending = true
		power = &m.PowerTemporary
		defer func() {
			if cause == nil {
				cause = m.finishPowerSource()
			}
		}()
	}
	if e := m.copyWithSourceContext(power, a, context); e != nil {
		return e
	}
	if e := m.setInt64WithSourceContext(b, 1, context); e != nil {
		return e
	}
	for mask := uint32(1); mask <= k; {
		if mask&k != 0 {
			if e := m.mulWithSourceContext(b, power, b, context); e != nil {
				return e
			}
		}
		if e := m.mulWithSourceContext(power, power, power, context); e != nil {
			return e
		}
		mask <<= 1
		if mask == 0 {
			return &PowerDomainError{Exponent: k, Reason: PowerMaskOverflow}
		}
	}
	if borrowed != nil {
		return borrowed.NumericPowerFinish()
	}
	if finish != nil {
		return finish()
	}
	return nil
}
func (m *IntegerModularManager) IsPerfectSquare(a, out *Int) (bool, error) {
	return m.isPerfectSquareWithSourceContext(a, out, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) isPerfectSquareWithSourceContext(a, out *Int, context integerAllocationInvocationContext) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if context.Caller != nil && context.roots(m.manager) == nil {
		return false, ErrModularOwner
	}
	if !m.z {
		return false, diagnostic.BorrowProcessDebugContext().NotImplementedYet("src/util/mpzzp.h", 261)
	}
	return m.manager.isPerfectSquareWithAllocationContext(a, out, context)
}
func (m *IntegerModularManager) Root(a *Int, n uint32, out *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNotModular(280); e != nil {
		return false, e
	}
	return m.manager.Root(a, n, out)
}
func (m *IntegerModularManager) RootInPlace(a *Int, n uint32) (bool, error) {
	return m.rootInPlaceWithReceipt(a, n, nil)
}
func (m *IntegerModularManager) rootInPlaceWithReceipt(a *Int, n uint32, receipt *integerRootInvocationReceipt) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNotModular(279); e != nil {
		return false, e
	}
	return m.manager.rootInPlaceWithReceipt(a, n, receipt)
}

// borrowedModulus mirrors source p(): private non-owning actual numeral.
// The caller must hold this manager lifetime and must not mutate/delete it.
func (m *IntegerModularManager) BorrowedModulus() (*Int, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	return &m.P, nil
}

// Source: src/math/polynomial/upolynomial.cpp:1108-1225,1734-1820,1906-1951,2029-2074;
// src/math/polynomial/algebraic_numbers.cpp:1565-1577;
// src/util/mpzzp.h:97-105. A returning transport must not end the actual
// modular owner while its registered source/NUM-destructor caller is executing.
func (m *IntegerModularManager) numericCallerExecuting() bool {
	return m != nil && m.self == m && (m.CallerExecuting != nil && m.CallerExecuting() || callerBindingsExecuting(m.callerBindings))
}

// IsPerfectSquareWithReceipt issues a receipt only when the six source RAW numerals are reached.
func (m *IntegerModularManager) IsPerfectSquareWithReceipt(a, out *Int, receipt *integerPerfectSquareInvocationReceipt) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if !m.z {
		return false, diagnostic.BorrowProcessDebugContext().NotImplementedYet("src/util/mpzzp.h", 261)
	}
	return m.manager.isPerfectSquareTracked(a, out, integerAllocationInvocationContext{}, receipt)
}

// An actual scoped_set_zp continuation may restore mode under its own borrow.
// Ordinary calls, stale generations and callbacks retain the public refusal.
func (m *IntegerModularManager) acceptedModeContext(c integerAllocationInvocationContext) bool {
	if !c.callerCurrent() || c.Caller == nil {
		return false
	}
	p, ok := c.Caller.Policy.(NumericCallerModularPolicy)
	return ok && p.NumericModularOwner(m) && c.roots(m.manager) != nil
}
