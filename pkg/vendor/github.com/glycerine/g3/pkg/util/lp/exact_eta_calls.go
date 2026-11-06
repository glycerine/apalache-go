package lp

// Source: src/util/lp/sparse_vector.h and exact eta division applications.

import "github.com/glycerine/g3/pkg/util/numeral"

// Precise matrix and indexed-vector normalization calls can retain integer
// GCD/division locals; matrix division may also alias its divisor and extrema
// start exact comparisons. Retain only newly acquired raw generations
// separately from the shared vector arithmetic receipts.
type exactEtaCalls struct {
	numeric              exactVectorCalls
	division, comparison uint64
	integerDivision, gcd numeral.IntegerRawInvocationReceipt
}
type exactEtaBefore struct {
	numeric              exactVectorBefore
	division             numeral.RationalDivisionInvocationState
	comparison           numeral.RationalComparisonInvocationState
	integerDivision, gcd numeral.IntegerRawInvocationReceipt
}

func (c *exactEtaCalls) pending() bool {
	return c.comparison != 0 || c.division != 0 || c.numeric.pending() || c.integerDivision.Invocation != 0 || c.gcd.Invocation != 0
}
func (c *exactEtaCalls) begin(m *numeral.RationalManager) (exactEtaBefore, error) {
	if c.pending() {
		return exactEtaBefore{}, ErrExactVectorPending
	}
	b, err := c.numeric.begin(m)
	return exactEtaBefore{numeric: b, division: m.DivisionInvocationState(), comparison: m.ComparisonInvocationState(), integerDivision: m.Z.DivisionInvocation(), gcd: m.Z.GCDInvocation()}, err
}
func (c *exactEtaCalls) after(m *numeral.RationalManager, b exactEtaBefore, cause error) error {
	d, g := m.Z.DivisionInvocation(), m.Z.GCDInvocation()
	if d.Matches(m.Z) && d.Invocation != b.integerDivision.Invocation {
		c.integerDivision = d
	}
	if g.Matches(m.Z) && g.Invocation != b.gcd.Invocation {
		c.gcd = g
	}
	comparison := m.ComparisonInvocationState()
	if comparison.Active && (!b.comparison.Active || comparison.Generation != b.comparison.Generation) {
		c.comparison = comparison.Generation
	}
	now := m.DivisionInvocationState()
	if now.Active && (!b.division.Active || now.Generation != b.division.Generation) {
		c.division = now.Generation
	}
	return c.numeric.after(m, b.numeric, cause)
}
func (c *exactEtaCalls) cleanupRaw(m *numeral.RationalManager) error {
	if err := m.CleanupComparisonInvocation(&c.comparison); err != nil {
		return err
	}
	if err := m.Z.CleanupRawInvocation(&c.integerDivision, true); err != nil {
		return err
	}
	c.integerDivision = numeral.IntegerRawInvocationReceipt{}
	if err := m.Z.CleanupRawInvocation(&c.gcd, true); err != nil {
		return err
	}
	c.gcd = numeral.IntegerRawInvocationReceipt{}
	if err := m.CleanupDivisionInvocation(&c.division); err != nil {
		return err
	}
	return c.numeric.cleanupRaw(m)
}
func (c *exactEtaCalls) cleanupRoots(m *numeral.RationalManager) error {
	return c.numeric.cleanupRoots(m)
}
