package numeral

// Source: src/util/mpbq.h/.cpp.

func (m *BinaryRationalManager) RootLower(a *BinaryRational, n uint32, out *BinaryRational) (bool, error) {
	return m.RootLowerWithInvocation(a, n, out, nil)
}
func (m *BinaryRationalManager) RootUpper(a *BinaryRational, n uint32, out *BinaryRational) (bool, error) {
	return m.RootUpperWithInvocation(a, n, out, nil)
}
func (m *BinaryRationalManager) RootLowerInPlace(a *BinaryRational, n uint32) (bool, error) {
	return m.root(a, n, true)
}
func (m *BinaryRationalManager) RootUpperInPlace(a *BinaryRational, n uint32) (bool, error) {
	return m.root(a, n, false)
}
func (m *BinaryRationalManager) root(a *BinaryRational, n uint32, lower bool) (bool, error) {
	return m.rootWithReceipt(a, n, lower, nil)
}
func (m *BinaryRationalManager) rootWithReceipt(a *BinaryRational, n uint32, lower bool, receipt *IntegerRootInvocationReceipt) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if n == 0 {
		return false, ErrBinaryRootDegreeZero
	}
	exact, e := m.Z.rootInPlaceWithReceipt(&a.Num, n, receipt)
	if e != nil {
		return false, e
	}
	if lower && !exact {
		if e = m.Z.Dec(&a.Num); e != nil {
			return false, e
		}
	}
	if a.KField%n == 0 {
		a.KField /= n
		return exact, m.normalize(a)
	}
	neg := a.Num.Val < 0
	a.KField /= n
	if lower && !neg || !lower && neg {
		a.KField++
	}
	return false, m.normalize(a)
}
func (m *BinaryRationalManager) Floor(a *BinaryRational, out *Int) error {
	if e := m.open(); e != nil {
		return e
	}
	return m.FloorInto(a, m.Z, out)
}

// FloorWithCopyAllocationReceipt records only the actual numerator-copy
// allocation; the later shift and negative-floor correction retain their bodies.
func (m *BinaryRationalManager) FloorWithCopyAllocationReceipt(a *BinaryRational, out *Int, receipt *IntegerCopyAllocationReceipt) error {
	if e := m.open(); e != nil {
		return e
	}
	return m.roundWithCopyAllocationReceipt(a, m.Z, out, false, receipt)
}
func (m *BinaryRationalManager) Ceil(a *BinaryRational, out *Int) error {
	if e := m.open(); e != nil {
		return e
	}
	return m.CeilInto(a, m.Z, out)
}
func (m *BinaryRationalManager) FloorInto(a *BinaryRational, z *IntegerManager, out *Int) error {
	return m.round(a, z, out, false)
}
func (m *BinaryRationalManager) CeilInto(a *BinaryRational, z *IntegerManager, out *Int) error {
	return m.round(a, z, out, true)
}
func (m *BinaryRationalManager) round(a *BinaryRational, z *IntegerManager, out *Int, ceil bool) (err error) {
	return m.roundWithCopyAllocationReceipt(a, z, out, ceil, nil)
}
func (m *BinaryRationalManager) roundWithCopyAllocationReceipt(a *BinaryRational, z *IntegerManager, out *Int, ceil bool, receipt *IntegerCopyAllocationReceipt) (err error) {
	return m.roundWithAllocationContext(a, z, out, ceil, receipt, integerAllocationInvocationContext{})
}
func (m *BinaryRationalManager) roundWithAllocationContext(a *BinaryRational, z *IntegerManager, out *Int, ceil bool, receipt *IntegerCopyAllocationReceipt, context integerAllocationInvocationContext) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	if z == nil || z.closed {
		return ErrBinaryOwner
	}
	if e := z.check(out); e != nil {
		return e
	}
	input := &a.Num
	if z != m.Z {
		m.roundImported = Int{}
		m.roundImportedOwner = z
		m.roundImportedPending = true
		m.roundExecuting = true
		defer func() {
			if cleanup := m.cleanupRoundImported(); err == nil {
				err = cleanup
			}
			m.roundExecuting = false
		}()
		if e := z.CopyFrom(&m.roundImported, m.Z, input); e != nil {
			return e
		}
		input = &m.roundImported
	}
	if a.KField == 0 {
		return z.CopyFromOwnerWithReceipt(out, z, input, receipt)
	}
	neg, pos := a.Num.Val < 0, a.Num.Val > 0
	if e := z.shiftWithAllocationContextAndReceipt(input, a.KField, out, false, integerAllocationInvocationContext{}, receipt); e != nil {
		return e
	}
	one := NewInt(1)
	if ceil && pos {
		return z.addSubWithAllocationContext(out, &one, out, false, context)
	}
	if !ceil && neg {
		return z.addSubWithAllocationContext(out, &one, out, true, context)
	}
	return nil
}

func (m *BinaryRationalManager) cleanupRoundImported() error {
	if !m.roundImportedPending {
		return nil
	}
	if e := m.roundImportedOwner.Del(&m.roundImported); e != nil {
		return e
	}
	m.roundImported = Int{}
	m.roundImportedOwner = nil
	m.roundImportedPending = false
	return nil
}

// CleanupRoundImported resumes release through the borrowed integer owner.
func (m *BinaryRationalManager) CleanupRoundImported() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.roundExecuting {
		return ErrBinaryRoundPending
	}
	return m.cleanupRoundImported()
}
func (m *BinaryRationalManager) FromRational(qm *RationalManager, q *Rational, out *BinaryRational) (exact bool, err error) {
	return m.FromRationalWithReceipt(qm, q, out, nil)
}
func (m *BinaryRationalManager) FromRationalWithReceipt(qm *RationalManager, q *Rational, out *BinaryRational, receipt *BinaryConvertInvocationReceipt) (exact bool, err error) {
	if e := m.check(out); e != nil {
		return false, e
	}
	if m.convertInvocation == ^uint64(0) {
		return false, ErrBinaryConvertPending
	}
	m.convertInvocation++
	m.convertSourceCause = nil
	m.convertDebugPending = false
	if receipt != nil {
		*receipt = BinaryConvertInvocationReceipt{m, m.convertInvocation}
	}
	m.convertImported = NewRational()
	m.convertImportedPending = true
	m.convertImportedCursor = 0
	m.convertExecuting = true
	defer func() {
		if cleanup := m.cleanupConvertImported(); cleanup != nil {
			m.convertSourceCause = cleanup
			if err == nil {
				err = cleanup
			}
		}
		m.convertDebugPending = m.toOwnedRationalScope.pending || m.debugIntegerPending || m.debugArithmeticPending || len(m.debugSnapshotQueue) != 0
		m.convertDebugChildren.capture(m)
		if m.convertSourceCause == nil {
			m.convertSourceCause = m.toOwnedRationalScope.sourceCause
		}
		if m.convertSourceCause == nil && m.debugArithmeticPending {
			m.convertSourceCause = m.debugArithmeticCall.SourceReady()
		}
		if m.convertSourceCause == nil && (m.debugIntegerPending || len(m.debugSnapshotQueue) != 0) {
			m.convertSourceCause = err
		}
		m.convertExecuting = false
	}()
	input, e := m.importRational(qm, q, &m.convertImported)
	if e != nil {
		return false, e
	}
	if isOneInteger(&input.Den) {
		if e = m.SetInteger(out, &input.Num); e != nil {
			return false, e
		}
		return true, m.debugRationalEqual(out, qm, q)
	}
	yes, k, e := m.Z.IsPowerOfTwo(&input.Den)
	if e != nil {
		return false, e
	}
	if yes {
		if e = m.debugAssert(k >= 1); e != nil {
			return false, e
		}
		if e = m.SetIntegerPower(out, &input.Num, k); e != nil {
			return false, e
		}
		return true, m.debugRationalEqual(out, qm, q)
	}
	k, e = m.Z.Log2(&input.Den)
	if e != nil {
		return false, e
	}
	return false, m.SetIntegerPower(out, &input.Num, k+1)
}

func (m *BinaryRationalManager) cleanupConvertImported() error {
	if !m.convertImportedPending {
		return nil
	}
	roots := [2]*Int{&m.convertImported.Num, &m.convertImported.Den}
	for m.convertImportedCursor < 2 {
		if e := m.Z.Del(roots[m.convertImportedCursor]); e != nil {
			return e
		}
		m.convertImportedCursor++
	}
	m.convertImported = Rational{}
	m.convertImportedCursor = 0
	m.convertImportedPending = false
	return nil
}

// CleanupConvertImported resumes release of a stopped Go cross-owner copy.
func (m *BinaryRationalManager) CleanupConvertImported() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.convertExecuting {
		return ErrBinaryConvertPending
	}
	return m.cleanupConvertImported()
}
func (m *BinaryRationalManager) ToRational(qm *RationalManager, a *BinaryRational, out *Rational) (err error) {
	return m.toRationalWithInvocation(qm, a, out, integerAllocationInvocationContext{}, nil)
}

// BinaryToRationalInvocation records actual RAW entries reached by to_mpq.
// The allocation context owns only storage issued during this invocation.
type BinaryToRationalInvocation struct {
	Power     RationalPowerInvocationReceipt
	Normalize RationalNormalizeInvocationReceipt
}

func (m *BinaryRationalManager) ToRationalWithInvocation(qm *RationalManager, a *BinaryRational, out *Rational, context NumericAllocationContext, receipt *BinaryToRationalInvocation) error {
	return m.toRationalWithInvocation(qm, a, out, context, receipt)
}
func (m *BinaryRationalManager) toRationalWithInvocation(qm *RationalManager, a *BinaryRational, out *Rational, context integerAllocationInvocationContext, receipt *BinaryToRationalInvocation) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	if qm == nil || qm.Z == nil {
		return ErrBinaryOwner
	}
	if e := qm.check(out); e != nil {
		return e
	}
	var power *RationalPowerInvocationReceipt
	var normalize *RationalNormalizeInvocationReceipt
	if receipt != nil {
		power = &receipt.Power
		normalize = &receipt.Normalize
	}
	two := Rational{Num: NewInt(2), Den: NewInt(1)}
	if e := qm.powerWithAllocationContextAndReceipt(&two, a.KField, out, context, power); e != nil {
		return e
	}
	if e := qm.invInPlaceWithAllocationContext(out, context); e != nil {
		return e
	}
	n := &a.Num
	if qm.Z != m.Z {
		m.toRationalImported = Int{}
		m.toRationalImportedOwner = qm.Z
		m.toRationalImportedPending = true
		m.toRationalExecuting = true
		defer func() {
			if cleanup := m.cleanupToRationalImported(); err == nil {
				err = cleanup
			}
			m.toRationalExecuting = false
		}()
		if e := qm.Z.CopyFrom(&m.toRationalImported, m.Z, n); e != nil {
			return e
		}
		n = &m.toRationalImported
	}
	return qm.mulIntWithAllocationContextAndReceipt(n, out, out, context, normalize)
}

func (m *BinaryRationalManager) cleanupToRationalImported() error {
	if !m.toRationalImportedPending {
		return nil
	}
	if e := m.toRationalImportedOwner.Del(&m.toRationalImported); e != nil {
		return e
	}
	m.toRationalImported = Int{}
	m.toRationalImportedOwner = nil
	m.toRationalImportedPending = false
	return nil
}

// CleanupToRationalImported resumes release through the borrowed MPQ owner.
func (m *BinaryRationalManager) CleanupToRationalImported() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.toRationalExecuting {
		return ErrBinaryToRationalPending
	}
	return m.cleanupToRationalImported()
}
func (m *BinaryRationalManager) ToOwnedRational(s *RationalService, a *BinaryRational) (out OwnedRational, err error) {
	if e := m.check(a); e != nil {
		return OwnedRational{}, e
	}
	if s == nil {
		return OwnedRational{}, ErrBinaryOwner
	}
	if m.toOwnedInvocation == ^uint64(0) {
		return OwnedRational{}, ErrBinaryOwnedRationalPending
	}
	m.toOwnedInvocation++
	scope := &m.toOwnedRationalScope
	*scope = binaryOwnedRationalScope{service: s, pending: true, executing: true}
	stopped := false
	defer func() {
		if !stopped {
			cleanup := m.cleanupToOwnedRational(4)
			if err == nil {
				err = cleanup
			}
			if cleanup == nil {
				if err == nil {
					out = scope.result
					scope.result = OwnedRational{}
					*scope = binaryOwnedRationalScope{}
				} else {
					_ = m.cleanupToOwnedRational(5)
				}
			}
		}
		scope.executing = false
	}()
	var e error
	scope.r, e = s.NewValue()
	if e != nil {
		return OwnedRational{}, e
	}
	if e = s.lock(); e != nil {
		return OwnedRational{}, e
	}
	e = s.Manager.Z.CopyFrom(&scope.r.Raw.Num, m.Z, &a.Num)
	s.mu.Unlock()
	if e != nil {
		return OwnedRational{}, e
	}
	scope.twok, e = s.NewValue()
	if e != nil {
		return OwnedRational{}, e
	}
	scope.two, e = s.NewValue()
	if e != nil {
		return OwnedRational{}, e
	}
	if e = s.SetInt64(&scope.two, 2); e != nil {
		return OwnedRational{}, e
	}
	scope.callsPending[0] = true
	scope.pw, e = s.PowerWithInvocation(&scope.two, a.KField, &scope.calls[0])
	if e != nil {
		return OwnedRational{}, e
	}
	if e = scope.finishCall(0); e != nil {
		return OwnedRational{}, e
	}
	scope.callsPending[1] = true
	if e = s.CopyWithInvocation(&scope.twok, &scope.pw, &scope.calls[1]); e != nil {
		return OwnedRational{}, e
	}
	if e = scope.finishCall(1); e != nil {
		return OwnedRational{}, e
	}
	if e = m.cleanupToOwnedRational(2); e != nil {
		stopped = true
		return OwnedRational{}, e
	}
	scope.callsPending[2] = true
	scope.result, e = s.DivWithInvocation(&scope.r, &scope.twok, &scope.calls[2])
	if e != nil {
		return OwnedRational{}, e
	}
	return OwnedRational{}, scope.finishCall(2)
}

func (s *binaryOwnedRationalScope) finishCall(i int) error {
	if e := s.calls[i].Cleanup(false); e != nil {
		return e
	}
	if e := s.calls[i].Cleanup(true); e != nil {
		return e
	}
	if e := s.calls[i].EndContext(); e != nil {
		return e
	}
	s.callsPending[i] = false
	return nil
}

func (m *BinaryRationalManager) cleanupToOwnedRational(limit uint8) error {
	scope := &m.toOwnedRationalScope
	if !scope.pending {
		return nil
	}
	if !scope.owning {
		for i := len(scope.callsPending) - 1; i >= 0; i-- {
			if scope.callsPending[i] {
				if e := scope.calls[i].SourceReady(); e != nil {
					scope.sourceCause = e
					return e
				}
			}
		}
	}
	roots := [5]*OwnedRational{&scope.pw, &scope.two, &scope.twok, &scope.r, &scope.result}
	for scope.cursor < limit {
		v := roots[scope.cursor]
		if v.ownerID != 0 {
			if e := scope.service.Del(v); e != nil {
				scope.sourceCause = e
				return e
			}
		}
		scope.cursor++
	}
	if scope.cursor == 5 {
		pending := false
		for _, call := range scope.callsPending {
			pending = pending || call
		}
		if !pending {
			*scope = binaryOwnedRationalScope{}
		}
	}
	return nil
}

// CleanupToOwnedRational resumes a stopped debug conversion's exact suffix.
func (m *BinaryRationalManager) CleanupToOwnedRational() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.toOwnedRationalScope.executing {
		return ErrBinaryOwnedRationalPending
	}
	scope := &m.toOwnedRationalScope
	scope.owning = true
	defer func() { scope.owning = false }()
	for i := len(scope.callsPending) - 1; i >= 0; i-- {
		if scope.callsPending[i] {
			if e := scope.calls[i].Cleanup(false); e != nil {
				return e
			}
		}
	}
	if e := m.cleanupToOwnedRational(5); e != nil {
		return e
	}
	for i := len(scope.callsPending) - 1; i >= 0; i-- {
		if scope.callsPending[i] {
			if e := scope.finishCall(i); e != nil {
				return e
			}
		}
	}
	*scope = binaryOwnedRationalScope{}
	return nil
}
