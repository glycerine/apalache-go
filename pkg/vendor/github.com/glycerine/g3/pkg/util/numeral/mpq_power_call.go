package numeral

// Source: mpq.cpp power and its reached normalization/integer children.

// RationalPowerInvocation owns only work accepted by this concrete power call.
// A failed body does not execute the source's RAW end labels.
type RationalPowerInvocation struct {
	self          *RationalPowerInvocation
	manager       *RationalManager
	power         RationalPowerInvocationReceipt
	normalize     RationalNormalizeInvocationReceipt
	division, gcd integerRawInvocationReceipt
	invocation    uint64
	hooks         NumericInvocationHooks
	roots         NumericAllocationRoots
	executing     bool
}

func (r *RationalPowerInvocation) valid() bool {
	return r != nil && r.self == r && r.manager != nil && r.manager.self == r.manager
}
func (r *RationalPowerInvocation) NumericBound(executing bool) bool {
	return r.valid() && (!executing || r.executing)
}
func (r *RationalPowerInvocation) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !r.valid() || !r.executing || z != r.manager.Z {
		return nil
	}
	if local != nil {
		return local
	}
	return &r.roots
}
func (r *RationalPowerInvocation) NumericRecord(z *IntegerManager, event string, receipt NumericInvocationReceipt) {
	if !r.valid() || !r.executing || z != r.manager.Z || receipt.Raw == nil {
		return
	}
	switch event {
	case "division":
		r.division = *receipt.Raw
	case "gcd":
		r.gcd = *receipt.Raw
	}
}
func (r *RationalPowerInvocation) Pending() bool {
	if !r.valid() {
		return false
	}
	m := r.manager
	return r.executing || m.MatchesRawPowerInvocation(&r.power) || m.MatchesRawNormalizeInvocation(&r.normalize) || m.Z.MatchesRawInvocation(&r.division) || m.Z.MatchesRawInvocation(&r.gcd) || NumericAllocationPending(&r.roots)
}
func (r *RationalPowerInvocation) SourceCause() error {
	if !r.valid() {
		return nil
	}
	if r.executing {
		return ErrNumericPending
	}
	m := r.manager
	for _, child := range []*integerRawInvocationReceipt{&r.division, &r.gcd} {
		if e := m.Z.RawInvocationSourceCause(child); e != nil {
			return e
		}
	}
	if e := m.RawNormalizeInvocationSourceCause(&r.normalize); e != nil {
		return e
	}
	return m.RawPowerInvocationSourceCause(&r.power)
}
func (r *RationalPowerInvocation) Cleanup(abort bool) error {
	if r == nil || r.self == nil {
		return nil
	}
	if !r.valid() {
		return ErrNumericOwner
	}
	if r.executing {
		return ErrNumericPending
	}
	m := r.manager
	for _, child := range []*integerRawInvocationReceipt{&r.division, &r.gcd} {
		if e := m.Z.CleanupRawInvocation(child, abort); e != nil {
			return e
		}
	}
	if abort || m.RawArithmetic.normEnd {
		if e := m.CleanupRawNormalizeInvocation(&r.normalize); e != nil {
			return e
		}
	}
	if abort || m.RawArithmetic.powerEnd {
		if e := m.CleanupRawPowerInvocation(&r.power); e != nil {
			return e
		}
	}
	if abort {
		return r.roots.Cleanup(m.Z)
	}
	return nil
}

func (m *RationalManager) PowerWithInvocation(a *Rational, exponent uint32, out *Rational, r *RationalPowerInvocation) error {
	if r == nil {
		return m.Power(a, exponent, out)
	}
	if r.self != nil && r.self != r {
		return ErrNumericOwner
	}
	if r.Pending() {
		return ErrNumericPending
	}
	if r.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	if e := m.checkScalarValue(out); e != nil {
		return e
	}
	*r = RationalPowerInvocation{manager: m, executing: true, invocation: r.invocation + 1}
	r.self = r
	r.hooks.Policy = r
	r.hooks.Generation = r.invocation
	context := NumericAllocationContext{}.WithCaller(&r.hooks)
	context.rationalPower = r
	defer func() { r.executing = false }()
	return m.powerWithAllocationContextAndReceipt(a, exponent, out, context, &r.power)
}
