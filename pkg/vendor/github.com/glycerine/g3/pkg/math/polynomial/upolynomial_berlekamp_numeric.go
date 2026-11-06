package polynomial

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Concrete ownership for the actual scalar calls in matrix construction and
// diagonalization. A fresh generation is admitted only after older work ends.
type upolyBerlekampNumeric struct {
	defaultNormalization upolyDefaultNormalizeReceipt
	owner                *upolyBerlekampMatrix
	hooks                unumeral.NumericInvocationHooks
	roots                unumeral.NumericAllocationRoots
	generation           uint64
	executing, cleaning  bool
	raw                  [3]unumeral.IntegerRawInvocationReceipt
	order                [3]uint8
	count                uint8
	addMul               unumeral.IntegerAddMulInvocationReceipt
	normalize            unumeral.IntegerNormalizeInvocationReceipt
	divisionAM           upolyBerlekampScopedNumeral
	monic                upolyBerlekampMonic
}

func (n *upolyBerlekampNumeric) NumericBound(executing bool) bool {
	q := n.owner
	return q != nil && q.m != nil && q.m.self == q.m && !q.m.closed && !q.m.closing && &q.numeric == n && n.generation != 0 && n.hooks.Generation == n.generation && (!executing || n.executing && !n.cleaning && q.m.callbackDepth == 0 && !q.m.config.Integer.RuntimeOwner().FailureTransportActive() && !q.m.config.Integer.BorrowTraceTags().OutputCallbackActive())
}
func (n *upolyBerlekampNumeric) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !n.NumericBound(true) || n.owner.m.config.Integer != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &n.roots
}
func (n *upolyBerlekampNumeric) NumericModularOwner(m *unumeral.IntegerModularManager) bool {
	return n.NumericBound(true) && n.owner.m.modular == m
}
func (n *upolyBerlekampNumeric) NumericRecord(z *unumeral.IntegerManager, _ string, r unumeral.NumericInvocationReceipt) {
	if !n.NumericBound(true) || n.owner.m.config.Integer != z || r.Raw == nil || !r.Raw.Matches(z) {
		return
	}
	var index uint8
	switch r.Raw.Kind() {
	case unumeral.IntegerRawDivision:
		index = 0
	case unumeral.IntegerRawGCD:
		index = 1
	case unumeral.IntegerRawExtendedGCD:
		index = 2
	default:
		return
	}
	n.raw[index] = *r.Raw
	for i := uint8(0); i < n.count; i++ {
		if n.order[i] == index {
			copy(n.order[i:n.count-1], n.order[i+1:n.count])
			n.count--
			break
		}
	}
	n.order[n.count] = index
	n.count++
}
func (n *upolyBerlekampNumeric) NumericNormalize(m *unumeral.IntegerModularManager, a unumeral.Int) (unumeral.Int, error) {
	if !n.NumericModularOwner(m) {
		return a, ErrUPolynomialOwner
	}
	e := m.PNormalizeWithFactorHistory(&a, &n.normalize, unumeral.NumericAllocationContext{}.WithCaller(&n.hooks))
	return a, e
}
func (n *upolyBerlekampNumeric) pending() bool {
	if n.owner == nil {
		return false
	}
	m := n.owner.m
	if m.defaultNormalizationReceiptPending(&n.defaultNormalization) {
		return true
	}
	z := m.config.Integer
	if n.monic.declared && n.monic.sourceIndex < 2 && !n.monic.ownEnded {
		return true
	}
	if n.divisionAM.declared && !n.divisionAM.sourceEnded && !n.divisionAM.ownEnded {
		return true
	}
	if unumeral.NumericAllocationPending(&n.roots) || n.addMul.Matches(z) || n.normalize.Matches(m.modular) {
		return true
	}
	for i := uint8(0); i < n.count; i++ {
		if n.raw[n.order[i]].Matches(z) {
			return true
		}
	}
	return false
}
func (n *upolyBerlekampNumeric) sourceCause() error {
	if n.owner == nil {
		return nil
	}
	if n.owner.m.defaultNormalizationReceiptPending(&n.defaultNormalization) {
		if e := n.owner.m.defaultNormalization.sourceCause; e != nil {
			return e
		}
	}
	if n.divisionAM.declared && !n.divisionAM.sourceEnded && !n.divisionAM.ownEnded && n.divisionAM.cause != nil {
		return n.divisionAM.cause
	}
	if n.monic.declared && n.monic.sourceIndex < 2 && !n.monic.ownEnded && n.monic.cause != nil {
		return n.monic.cause
	}
	z := n.owner.m.config.Integer
	if n.addMul.Matches(z) {
		if e := n.addMul.SourceCause(); e != nil {
			return e
		}
	}
	for i := int(n.count) - 1; i >= 0; i-- {
		r := &n.raw[n.order[i]]
		if r.Matches(z) {
			if e := z.RawInvocationSourceCause(r); e != nil {
				return e
			}
		}
	}
	return nil
}
func (n *upolyBerlekampNumeric) cleanup(abort bool) error {
	if n.owner == nil {
		return nil
	}
	if n.executing || n.cleaning {
		return ErrUPolynomialCleanupPending
	}
	n.cleaning = true
	defer func() { n.cleaning = false }()
	if e := n.owner.m.cleanupDefaultNormalizationReceipt(&n.defaultNormalization); e != nil {
		return e
	}
	z := n.owner.m.config.Integer
	if e := n.owner.m.modular.CleanupPNormalizeInvocation(&n.normalize, abort); e != nil {
		return e
	}
	if e := z.CleanupAddMulInvocation(&n.addMul, abort); e != nil {
		return e
	}
	for i := int(n.count) - 1; i >= 0; i-- {
		if e := z.CleanupRawInvocation(&n.raw[n.order[i]], abort); e != nil {
			return e
		}
	}
	if abort {
		return n.roots.Cleanup(z)
	}
	return nil
}

// Each admitted scalar or vector call borrows this exact matrix owner.
func (q *upolyBerlekampMatrix) beginNumeric(borrow *upolyFactorTraceBorrow) error {
	n := &q.numeric
	if n.executing || n.cleaning || n.pending() || n.generation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	n.owner = q
	n.generation++
	n.hooks.Policy = n
	n.hooks.Generation = n.generation
	n.executing = true
	if e := borrow.bindManagers([3]*UPolynomialManager{q.m}, q.m.modular.BorrowTraceTags()); e != nil {
		n.executing = false
		return e
	}
	return nil
}
func (q *upolyBerlekampMatrix) endNumeric(borrow *upolyFactorTraceBorrow, cause error) {
	n := &q.numeric
	borrow.unbind()
	n.executing = false
	if n.pending() {
		if q.m.pendingBerlekamp == nil || q.m.pendingBerlekamp == q {
			q.m.pendingBerlekamp = q
		}
		if q.closeCause == nil {
			q.closeCause = n.sourceCause()
			if q.closeCause == nil {
				q.closeCause = cause
			}
		}
	}
}

// op selects only concrete numeral operations reached by this matrix.
func (q *upolyBerlekampMatrix) numericCall(op uint8, a, b, c, out *unumeral.Int) (cause error) {
	var borrow upolyFactorTraceBorrow
	if e := q.beginNumeric(&borrow); e != nil {
		return e
	}
	defer func() { q.endNumeric(&borrow, cause) }()
	n := &q.numeric
	ops := q.m.modular.WithAllocationContext(unumeral.NumericAllocationContext{}.WithCaller(&n.hooks))
	switch op {
	case 0:
		return ops.Copy(out, a)
	case 1:
		return ops.SubMul(a, b, c, out, &n.addMul)
	case 2:
		return ops.Mul(a, b, out)
	case 3:
		return ops.Neg(out)
	case 4:
		return ops.Inv(out)
	case 5:
		return ops.AddMulTracked(a, b, c, out, &n.addMul)
	case 6:
		return ops.Dec(out)
	default:
		return ErrUPolynomialOwner
	}
}

// Actual live callers at factorization.cpp:474 and :480. Their scalar leaves
// share the matrix's concrete ownership policy, not an ambient manager scan.
func (q *upolyBerlekampMatrix) vectorNumericCall(divide bool, a, b UPolynomialView, out *upolyVectorState) (cause error) {
	var borrow upolyFactorTraceBorrow
	if e := q.beginNumeric(&borrow); e != nil {
		return e
	}
	defer func() { q.endNumeric(&borrow, cause) }()
	context := integerAllocationInvocationContext{berlekamp: &q.numeric, berlekampGeneration: q.numeric.generation}
	if divide {
		return q.m.divStateWithSourceContext(a, b, out, context)
	}
	return q.m.gcdWithSourceContext(a, b, out, context)
}
