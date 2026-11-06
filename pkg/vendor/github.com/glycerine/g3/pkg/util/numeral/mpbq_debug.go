//go:build g3debug

package numeral

// Source MPBQ_DEBUG rational actions.

func (m *BinaryRationalManager) debugSelected(l, u, r *BinaryRational) error {
	yes, e := m.LessEqual(l, r)
	if e != nil {
		return e
	}
	if e = m.debugAssert(yes); e != nil {
		return e
	}
	yes, e = m.LessEqual(r, u)
	if e != nil {
		return e
	}
	return m.debugAssert(yes)
}
func (m *BinaryRationalManager) debugEnclosure(qm *RationalManager, q *Rational, l, u *BinaryRational) error {
	lo, e := m.LessRational(l, qm, q)
	if e != nil {
		return e
	}
	if !lo {
		return ErrBinaryInvariant
	}
	hi, e := m.GreaterRational(u, qm, q)
	if e != nil {
		return e
	}
	return m.debugAssert(lo && hi)
}
func (m *BinaryRationalManager) debugRationalEqual(a *BinaryRational, qm *RationalManager, q *Rational) error {
	yes, e := m.EqualRational(a, qm, q)
	if e != nil {
		return e
	}
	return m.debugAssert(yes)
}

func binaryCheckDiagnostics(z *IntegerManager, s *RationalService) error {
	if s == nil || s.self != s || s.closed || s.Manager == nil || s.Manager.Budget() != z.Budget() {
		return ErrBinaryOwner
	}
	return nil
}
func (m *BinaryRationalManager) debugAssert(ok bool) error {
	if !ok {
		return ErrBinaryInvariant
	}
	return nil
}
func (m *BinaryRationalManager) debugValue(a *BinaryRational) (OwnedRational, error) {
	return m.ToOwnedRational(m.Diagnostics, a)
}
func (m *BinaryRationalManager) debugInteger(a *Int) (out OwnedRational, err error) {
	if e := m.open(); e != nil {
		return OwnedRational{}, e
	}
	if m.debugIntegerInvocation == ^uint64(0) {
		return OwnedRational{}, ErrBinaryDebugIntegerPending
	}
	m.debugIntegerInvocation++
	s := m.Diagnostics
	m.debugIntegerValue = OwnedRational{}
	m.debugIntegerPending = true
	m.debugIntegerExecuting = true
	defer func() {
		if err == nil {
			out = m.debugIntegerValue
			m.debugIntegerValue = OwnedRational{}
			m.debugIntegerPending = false
		} else {
			_ = m.cleanupDebugInteger()
		}
		m.debugIntegerExecuting = false
	}()
	var e error
	m.debugIntegerValue, e = s.NewValue()
	if e != nil {
		return OwnedRational{}, e
	}
	if e = s.lock(); e != nil {
		return OwnedRational{}, e
	}
	e = s.Manager.Z.CopyFrom(&m.debugIntegerValue.Raw.Num, m.Z, a)
	s.mu.Unlock()
	if e != nil {
		return OwnedRational{}, e
	}
	return OwnedRational{}, nil
}
func (m *BinaryRationalManager) debugDel(a *OwnedRational) error {
	if a.ownerID == 0 {
		return nil
	}
	if len(m.debugSnapshotQueue) != 0 {
		m.debugSnapshotQueue = append(m.debugSnapshotQueue, *a)
		*a = OwnedRational{}
		return ErrBinaryDebugSnapshotPending
	}
	if e := m.Diagnostics.Del(a); e != nil {
		m.debugSnapshotQueue = append(m.debugSnapshotQueue, *a)
		*a = OwnedRational{}
		return e
	}
	return nil
}
func (m *BinaryRationalManager) debugEqualValue(old *OwnedRational, a *BinaryRational) (err error) {
	v, e := m.debugValue(a)
	if e != nil {
		return e
	}
	defer func() {
		if cleanup := m.debugDel(&v); err == nil {
			err = cleanup
		}
	}()
	same, e := m.Diagnostics.Equal(old, &v)
	if e != nil {
		return e
	}
	return m.debugAssert(same)
}
func (m *BinaryRationalManager) debugArithmetic(a, b *OwnedRational, out *BinaryRational, op uint8, traceLine int) (err error) {
	r, e := m.debugValue(out)
	if e != nil {
		return e
	}
	defer func() {
		if cleanup := m.debugDel(&r); err == nil {
			err = cleanup
		}
	}()
	var want OwnedRational
	if m.debugArithmeticInvocation == ^uint64(0) {
		return ErrBinaryDebugSnapshotPending
	}
	m.debugArithmeticInvocation++
	m.debugArithmeticCall = RationalServiceInvocation{}
	m.debugArithmeticPending = true
	m.debugArithmeticExecuting = true
	defer func() { m.debugArithmeticExecuting = false }()
	switch op {
	case 0:
		want, e = m.Diagnostics.AddWithInvocation(a, b, &m.debugArithmeticCall)
	case 1:
		want, e = m.Diagnostics.SubWithInvocation(a, b, &m.debugArithmeticCall)
	case 2:
		want, e = m.Diagnostics.MulWithInvocation(a, b, &m.debugArithmeticCall)
	}
	if e != nil {
		return e
	}
	defer func() {
		if cleanup := m.debugDel(&want); err == nil {
			err = cleanup
		}
	}()
	if e = m.debugArithmeticCall.Cleanup(false); e != nil {
		return e
	}
	if e = m.debugArithmeticCall.Cleanup(true); e != nil {
		return e
	}
	if e = m.debugArithmeticCall.EndContext(); e != nil {
		return e
	}
	m.debugArithmeticPending = false
	if traceLine != 0 {
		m.traceMPBQArithmetic(a, b, &r, &want, traceLine)
	}
	same, e := m.Diagnostics.Equal(&want, &r)
	if e != nil {
		return e
	}
	return m.debugAssert(same)
}
func (m *BinaryRationalManager) debugApproxSnapshot(a, old *BinaryRational) error {
	return m.Copy(old, a)
}
func (m *BinaryRationalManager) debugApproxDrop(old *BinaryRational) error {
	if e := m.Del(old); e != nil {
		m.approxOld = *old
		*old = BinaryRational{}
		m.approxPending = true
		return e
	}
	return nil
}
func (m *BinaryRationalManager) debugApprox(old, a *BinaryRational, plus bool) error {
	var yes bool
	var e error
	if plus {
		yes, e = m.Less(old, a)
	} else {
		yes, e = m.Less(a, old)
	}
	if e != nil {
		return e
	}
	return m.debugAssert(yes)
}
