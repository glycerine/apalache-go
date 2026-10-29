package polynomial

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Caller algorithms keep their own scopes; primitive ownership helpers live in util.
type polynomialNumericRoots struct {
	operation                      polynomialNumeralOperationGuard
	integerBinding, naturalBinding unumeral.NumericCallerBinding
	unumeral.NumericAllocationRoots
	hooks   unumeral.NumericInvocationHooks
	caller  integerAllocationInvocationContext
	integer *unumeral.IntegerManager
}

func polynomialNumericBefore(z *unumeral.IntegerManager) (polynomialNumericRoots, error) {
	r, e := unumeral.CaptureNumericAllocationRoots(z)
	return polynomialNumericRoots{NumericAllocationRoots: r}, e
}
func polynomialNegNumericPending(r *polynomialNumericRoots) bool {
	return unumeral.NumericAllocationPending(&r.NumericAllocationRoots)
}
func polynomialNewtonJoin(a, b error) error { return unumeral.JoinNumericErrors(a, b) }
func upolynomialSourceNumeralDel(z *unumeral.IntegerManager, a *unumeral.Int) (bool, error) {
	return unumeral.SourceNumeralDel(z, a)
}

func (r *polynomialNumericRoots) After(z *unumeral.IntegerManager, before polynomialNumericRoots) error {
	return r.NumericAllocationRoots.After(z, before.NumericAllocationRoots)
}
func (r *polynomialNumericRoots) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if r == nil || r.integer != z {
		return nil
	}
	current := r.caller.roots(z)
	if current != r {
		// The early constant SET starts with its embedded binding. A reached
		// allocation creates its stable call record during Prepare; the same
		// in-flight primitive context must then acquire into that exact record.
		b := r.caller.factorFrontendConstantFrame(z)
		if b == nil || &b.numeric != r || b.record == nil || current != &b.record.numeric {
			return nil
		}
	}
	if local != nil {
		return local
	}
	return &current.NumericAllocationRoots
}
func (c integerAllocationInvocationContext) utility(z *unumeral.IntegerManager) unumeral.NumericAllocationContext {
	if c.squareFreeCall != nil {
		q := c.squareFreeCallFrame(z)
		if q == nil {
			return unumeral.NumericAllocationContext{}.WithCaller(&invalidPolynomialNumericHooks)
		}
		return unumeral.NumericAllocationContext{}.WithCaller(&q.hooks)
	}
	if c.defaultNormalization != nil {
		q := c.defaultNormalizationFrame(z)
		if q == nil {
			return unumeral.NumericAllocationContext{}.WithCaller(&invalidPolynomialNumericHooks)
		}
		return unumeral.NumericAllocationContext{}.WithCaller(&q.hooks)
	}
	if n := c.berlekamp; n != nil {
		if !n.NumericBound(true) || n.owner.m.config.Integer != z || c.berlekampGeneration == 0 || c.berlekampGeneration != n.generation {
			return unumeral.NumericAllocationContext{}.WithCaller(&invalidPolynomialNumericHooks)
		}
		return unumeral.NumericAllocationContext{}.WithCaller(&n.hooks)
	}
	if c.sexprArithmetic != nil {
		q := c.sexprArithmeticFrame(z)
		if q == nil {
			return unumeral.NumericAllocationContext{}.WithCaller(&invalidPolynomialNumericHooks)
		}
		return unumeral.NumericAllocationContext{}.WithCaller(&q.hooks)
	}
	if c.sturmIsolation != nil {
		q := c.sturmIsolationFrame(z)
		if q == nil {
			return unumeral.NumericAllocationContext{}.WithCaller(&invalidPolynomialNumericHooks)
		}
		return unumeral.NumericAllocationContext{}.WithCaller(&q.hooks)
	}

	if c.sequenceSturm != nil {
		q := c.sequenceSturmFrame(z)
		if q == nil {
			return unumeral.NumericAllocationContext{}.WithCaller(&invalidPolynomialNumericHooks)
		}
		return unumeral.NumericAllocationContext{}.WithCaller(&q.hooks)
	}
	if c.rootID != nil {
		q := c.rootIDFrame(z)
		if q == nil {
			return unumeral.NumericAllocationContext{}.WithCaller(&invalidPolynomialNumericHooks)
		}
		return unumeral.NumericAllocationContext{}.WithCaller(&q.hooks)
	}
	if c == (integerAllocationInvocationContext{}) {
		return unumeral.NumericAllocationContext{}
	}
	r := c.roots(z)
	if r == nil {
		return unumeral.NumericAllocationContext{}.WithCaller(&invalidPolynomialNumericHooks)
	}
	r.caller = c
	r.integer = z
	r.hooks.Policy = r
	return unumeral.NumericAllocationContext{}.WithCaller(&r.hooks)
}

var invalidPolynomialNumericHooks unumeral.NumericInvocationHooks

// The higher source frame retains the primitive allocation receipt.
func (r *polynomialNumericRoots) after(z *unumeral.IntegerManager, before polynomialNumericRoots) error {
	return r.After(z, before)
}
func (r *polynomialNumericRoots) cleanup(z *unumeral.IntegerManager) error {
	return r.NumericAllocationRoots.Cleanup(z)
}
