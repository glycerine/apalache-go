package polynomial

// Source: math/polynomial numerical calls; synchronous utility owner exclusion.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialNumeralOperationGuard struct {
	binding   unumeral.NumericCallerBinding
	executing bool
}

func (g *polynomialNumeralOperationGuard) NumericExecuting() bool { return g != nil && g.executing }
func (g *polynomialNumeralOperationGuard) enter(z *unumeral.IntegerManager) error {
	if g.executing {
		return ErrPolynomialPending
	}
	g.executing = true
	if e := z.BindCaller(&g.binding, g, 16); e != nil {
		g.executing = false
		return e
	}
	return nil
}
func (g *polynomialNumeralOperationGuard) leave() {
	if e := g.binding.Unbind(); e != nil {
		panic(e)
	}
	g.executing = false
}
func factorNumeralCallExecuting(z *unumeral.IntegerManager) bool {
	for b := z.CallerBindings(); b != nil; b = b.Previous() {
		if b.Kind() == 16 && b.Guard().NumericExecuting() {
			return true
		}
	}
	return false
}
