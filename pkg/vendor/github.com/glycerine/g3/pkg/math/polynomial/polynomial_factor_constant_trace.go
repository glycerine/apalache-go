package polynomial

// Source: src/math/polynomial/polynomial.cpp:6578.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

type polynomialFactorConstantTraceBorrow struct{ executing bool }

func (b *polynomialFactorConstantTraceBorrow) NumericExecuting() bool { return b.executing }
func (m *MultivariatePolynomialManager) traceAccumulateFactorConstant(f *PolynomialFactors, a *unumeral.Int, r *unumeral.NumeralInsertionReceipt) error {
	if !m.core.monomials.monomialTraceTags().ShouldTrace("factor_bug") {
		return nil
	}
	var borrow polynomialFactorConstantTraceBorrow
	borrow.executing = true
	var integer, natural unumeral.NumericCallerBinding
	if e := m.core.integer.BindCaller(&integer, &borrow, 0); e != nil {
		return e
	}
	defer func() {
		if e := integer.Unbind(); e != nil {
			panic(e)
		}
	}()
	if e := m.core.integer.Natural.BindCaller(&natural, &borrow, 0); e != nil {
		return e
	}
	defer func() {
		if e := natural.Unbind(); e != nil {
			panic(e)
		}
	}()
	return m.emitFactorTraceTag(f, "factor_bug", "acc_constant", 6578, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("acc_constant, c: ")
		if e := m.core.integer.DisplayToTracked(out, a, r); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
