package numeral

// Source: src/util/mpz.cpp; src/util/mpzzp.h (caller lifetime adaptation).

import (
	"github.com/glycerine/g3/pkg/arena"
)

// NumericIntegerOperations borrows a primitive manager and an explicit caller
// context. Its methods preserve the ordinary arithmetic and source epilogues.
// The caller owns the hooks and must keep their identity valid for the call.

type NumericIntegerOperations struct {
	manager *IntegerManager
	context NumericAllocationContext
}

func (m *IntegerManager) WithAllocationContext(c NumericAllocationContext) NumericIntegerOperations {
	return NumericIntegerOperations{m, c}
}
func (o NumericIntegerOperations) valid() error {
	m := o.manager
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if o.context != (NumericAllocationContext{}) && o.context.roots(m) == nil {
		return ErrNumericOwner
	}
	return nil
}
func (o NumericIntegerOperations) SetInt64(a *Int, v int64) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setInt64WithAllocationContext(a, v, o.context)
}
func (o NumericIntegerOperations) Copy(a, b *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.copyFromOwnerWithAllocationContext(a, o.manager, b, nil, o.context)
}
func (o NumericIntegerOperations) Add(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addSubWithAllocationContext(a, b, out, false, o.context)
}
func (o NumericIntegerOperations) Sub(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addSubWithAllocationContext(a, b, out, true, o.context)
}
func (o NumericIntegerOperations) Mul(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.mulWithAllocationContext(a, b, out, o.context)
}
func (o NumericIntegerOperations) Div(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.divWithAllocationContext(a, b, out, o.context)
}
func (o NumericIntegerOperations) Mod(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.modWithAllocationContext(a, b, out, o.context)
}
func (o NumericIntegerOperations) GCD(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.gcdWithAllocationContext(a, b, out, o.context)
}
func (o NumericIntegerOperations) ExtendedGCD(a, b, u, v, g *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.extendedGCDWithAllocationContext(a, b, u, v, g, o.context)
}
func (o NumericIntegerOperations) Divides(a, b *Int) (bool, error) {
	if e := o.valid(); e != nil {
		return false, e
	}
	return o.manager.dividesWithAllocationContext(a, b, o.context)
}
func (r *integerRawInvocationReceipt) Cleanup(abort bool) error {
	if r == nil {
		return nil
	}
	if r.Manager == nil {
		return ErrNumericOwner
	}
	return r.Manager.CleanupRawInvocation(r, abort)
}
func (r *integerDividesNumeralInvocationReceipt) Cleanup(abort bool) error {
	if r == nil {
		return nil
	}
	if r.Manager == nil {
		return ErrNumericOwner
	}
	return r.Manager.CleanupDividesNumeralInvocation(r, abort)
}
func (r *integerPerfectSquareInvocationReceipt) Cleanup(abort bool) error {
	if r == nil {
		return nil
	}
	if r.manager == nil {
		return ErrNumericOwner
	}
	return r.manager.CleanupPerfectSquareInvocation(r, abort)
}

// NumericModularOperations carries the same caller policy through modular
// arithmetic, including the source normalization and power scopes.
type NumericModularOperations struct {
	manager *IntegerModularManager
	context NumericAllocationContext
}

func (m *IntegerModularManager) WithAllocationContext(c NumericAllocationContext) NumericModularOperations {
	return NumericModularOperations{m, c}
}
func (o NumericModularOperations) valid() error {
	if o.manager == nil {
		return ErrModularOwner
	}
	if e := o.manager.check(); e != nil {
		return e
	}
	if o.context != (NumericAllocationContext{}) && o.context.roots(o.manager.manager) == nil {
		return ErrNumericOwner
	}
	if h := o.context.Caller; h != nil {
		if p, ok := h.Policy.(NumericCallerModularPolicy); ok && !p.NumericModularOwner(o.manager) {
			return ErrNumericOwner
		}
	}
	return nil
}
func (o NumericModularOperations) SetInt64(a *Int, v int64) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setInt64WithSourceContext(a, v, o.context)
}
func (o NumericModularOperations) Copy(a, b *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.copyWithSourceContext(a, b, o.context)
}
func (o NumericModularOperations) Add(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addWithSourceContext(a, b, out, o.context)
}
func (o NumericModularOperations) Sub(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.subWithSourceContext(a, b, out, o.context)
}
func (o NumericModularOperations) Mul(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.mulWithSourceContext(a, b, out, o.context)
}
func (o NumericModularOperations) AddMul(a, b, c, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addMulWithSourceContext(a, b, c, out, nil, o.context)
}
func (o NumericModularOperations) GCD(a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.gcdWithSourceContext(a, b, out, o.context)
}
func (o NumericModularOperations) ExtendedGCD(a, b, u, v, g *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.extendedGCDWithSourceContext(a, b, u, v, g, o.context)
}
func (o NumericModularOperations) Power(a *Int, k uint32, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.powerWithSourceContext(a, k, out, o.context)
}
func (o NumericModularOperations) IsPerfectSquare(a, out *Int) (bool, error) {
	if e := o.valid(); e != nil {
		return false, e
	}
	return o.manager.isPerfectSquareWithSourceContext(a, out, o.context)
}
func (o NumericModularOperations) Divides(a, b *Int) (bool, error) {
	if e := o.valid(); e != nil {
		return false, e
	}
	return o.manager.dividesWithAllocationContext(a, b, o.context)
}
func (o NumericModularOperations) SetPSquare() error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setPSquareWithAllocationContext(o.context)
}
func (o NumericModularOperations) Normalize(a *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.PNormalizeWithFactorHistory(a, nil, o.context)
}
func (o NumericIntegerOperations) AddMul(a, b, c, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addMulWithAllocationContext(a, b, c, out, nil, o.context)
}
func (o NumericIntegerOperations) IsPerfectSquare(a, out *Int) (bool, error) {
	if e := o.valid(); e != nil {
		return false, e
	}
	return o.manager.isPerfectSquareWithAllocationContext(a, out, o.context)
}
