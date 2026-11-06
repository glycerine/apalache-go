package numeral

// Source: src/util/mpzzp.h; src/util/mpzzp.cpp.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func (m *IntegerModularManager) sourceAssertFinite(line int32) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled || !m.z {
		return e
	}
	return diagnostic.DebugAssertionFailure(d, "src/util/mpzzp.h", line, "!m_z")
}

func (m *IntegerModularManager) sourceAssertNotModular(line int32) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled || m.z {
		return e
	}
	return diagnostic.DebugAssertionFailure(d, "src/util/mpzzp.h", line, "!modular()")
}

// These sites share the same predicate, but retain their distinct source
// locations and the left-before-right short circuit of binary assertions.
func (m *IntegerModularManager) sourceAssertNormalized(a *Int, line int32) error {
	return m.sourceAssertNormalizedNamed(a, line, "is_p_normalized(a)")
}

func (m *IntegerModularManager) sourceAssertNormalizedNamed(a *Int, line int32, condition string) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	ok, e := m.IsPNormalized(a)
	if e != nil || ok {
		return e
	}
	return diagnostic.DebugAssertionFailure(d, "src/util/mpzzp.h", line, condition)
}

func (m *IntegerModularManager) sourceAssertNormalizedPair(a, b *Int, line int32) error {
	return m.sourceAssertNormalizedPairNamed(a, b, line, "is_p_normalized(a) && is_p_normalized(b)")
}

func (m *IntegerModularManager) sourceAssertNormalizedPairNamed(a, b *Int, line int32, condition string) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	left, e := m.IsPNormalized(a)
	if e != nil {
		return e
	}
	ok := left
	if left {
		ok, e = m.IsPNormalized(b)
		if e != nil {
			return e
		}
	}
	if ok {
		return nil
	}
	return diagnostic.DebugAssertionFailure(d, "src/util/mpzzp.h", line, condition)
}

func (m *IntegerModularManager) sourceAssertNormalizedTriple(a, b, c *Int, line int32) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	for _, v := range []*Int{a, b, c} {
		ok, e := m.IsPNormalized(v)
		if e != nil {
			return e
		}
		if !ok {
			return diagnostic.DebugAssertionFailure(d, "src/util/mpzzp.h", line, "is_p_normalized(a) && is_p_normalized(b) && is_p_normalized(c)")
		}
	}
	return nil
}

func (m *IntegerModularManager) Reset(a *Int) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.Reset(a); e != nil {
		return e
	}
	return nil
}
func (m *IntegerModularManager) Del(a *Int) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.Del(a); e != nil {
		return e
	}
	return nil
}
func (m *IntegerModularManager) Neg(a *Int) error { return m.negWithPNReceipt(a, nil) }

// Source: src/util/mpzzp.h:131. Both concrete caller routes use this same
// mutation engine; a PN receipt is acquired only at its real reached guard.
func (m *IntegerModularManager) negWithPNReceipt(a *Int, receipt *integerPNormalizeInvocationReceipt) error {
	return m.negWithOwners(a, receipt, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) negWithSourceContext(a *Int, context integerAllocationInvocationContext) error {
	return m.negWithOwners(a, nil, context)
}
func (m *IntegerModularManager) negWithOwners(a *Int, receipt *integerPNormalizeInvocationReceipt, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.negWithAllocationContext(a, context); e != nil {
		return e
	}
	if receipt != nil {
		return m.PNormalizeWithReceipt(a, receipt)
	}
	return m.normalizeWithSourceContext(a, context)
}

func (m *IntegerModularManager) Abs(a *Int) error {
	return m.absWithSourceContext(a, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) absWithSourceContext(a *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.absWithAllocationContext(a, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(a, context)
}
func (m *IntegerModularManager) Inc(a *Int) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalized(a, 152); e != nil {
		return e
	}
	if e := m.manager.Inc(a); e != nil {
		return e
	}
	return m.PNormalize(a)
}
func (m *IntegerModularManager) Dec(a *Int) error {
	return m.decWithSourceContext(a, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) decWithSourceContext(a *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalized(a, 153); e != nil {
		return e
	}
	// Preserve IntegerManager.Dec's existing concrete Add(-1) engine while
	// carrying the retained caller to its actual allocation leaves.
	minusOne := NewInt(-1)
	if e := m.manager.addSubWithAllocationContext(a, &minusOne, a, false, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(a, context)
}
func (m *IntegerModularManager) Add(a, b, c *Int) error {
	return m.addWithSourceContext(a, b, c, integerAllocationInvocationContext{})
}

func (m *IntegerModularManager) addWithSourceContext(a, b, c *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 150); e != nil {
		return e
	}
	if e := m.manager.addSubWithAllocationContext(a, b, c, false, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(c, context)
}
func (m *IntegerModularManager) Sub(a, b, c *Int) error {
	return m.subWithSourceContext(a, b, c, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) subWithSourceContext(a, b, c *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 151); e != nil {
		return e
	}
	if e := m.manager.addSubWithAllocationContext(a, b, c, true, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(c, context)
}
func (m *IntegerModularManager) Mul(a, b, c *Int) error {
	return m.mulWithSourceContext(a, b, c, integerAllocationInvocationContext{})
}

func (m *IntegerModularManager) mulWithSourceContext(a, b, c *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 154); e != nil {
		return e
	}
	if e := m.manager.mulWithAllocationContext(a, b, c, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(c, context)
}
func (m *IntegerModularManager) AddMul(a, b, c, d *Int) error {
	return m.addMulWithSourceContext(a, b, c, d, nil, integerAllocationInvocationContext{})
}

func (m *IntegerModularManager) addMulWithSourceContext(a, b, c, d *Int, child *integerAddMulInvocationReceipt, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalizedTriple(a, b, c, 156); e != nil {
		return e
	}
	if e := m.manager.addMulWithAllocationContext(a, b, c, d, child, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(d, context)
}
func (m *IntegerModularManager) SubMul(a, b, c, d *Int) error {
	return m.subMulWithSourceContext(a, b, c, d, nil, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) subMulWithSourceContext(a, b, c, d *Int, child *integerAddMulInvocationReceipt, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalized(a, 160); e != nil {
		return e
	}
	if e := m.sourceAssertNormalizedNamed(b, 161, "is_p_normalized(b)"); e != nil {
		return e
	}
	if e := m.sourceAssertNormalizedNamed(c, 162, "is_p_normalized(c)"); e != nil {
		return e
	}
	if e := m.manager.addSubMulWithAllocationContext(a, b, c, d, true, child, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(d, context)
}
func (m *IntegerModularManager) IsZero(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalized(a, 133); e != nil {
		return false, e
	}
	return m.manager.IsZero(a)
}
func (m *IntegerModularManager) IsOne(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalized(a, 134); e != nil {
		return false, e
	}
	return m.manager.IsOne(a)
}
func (m *IntegerModularManager) IsPos(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalized(a, 135); e != nil {
		return false, e
	}
	return m.manager.IsPos(a)
}
func (m *IntegerModularManager) IsNeg(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalized(a, 136); e != nil {
		return false, e
	}
	return m.manager.IsNeg(a)
}
func (m *IntegerModularManager) IsNonPos(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalized(a, 137); e != nil {
		return false, e
	}
	return m.manager.IsNonPos(a)
}
func (m *IntegerModularManager) IsNonNeg(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalized(a, 138); e != nil {
		return false, e
	}
	return m.manager.IsNonNeg(a)
}
func (m *IntegerModularManager) IsMinusOne(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalized(a, 139); e != nil {
		return false, e
	}
	return m.manager.IsMinusOne(a)
}
func (m *IntegerModularManager) Equal(a, b *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 140); e != nil {
		return false, e
	}
	return m.manager.Equal(a, b)
}
func (m *IntegerModularManager) Less(a, b *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 141); e != nil {
		return false, e
	}
	return m.manager.Less(a, b)
}
func (m *IntegerModularManager) LessEqual(a, b *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 142); e != nil {
		return false, e
	}
	return m.manager.LessEqual(a, b)
}
func (m *IntegerModularManager) Greater(a, b *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 143); e != nil {
		return false, e
	}
	return m.manager.Greater(a, b)
}
func (m *IntegerModularManager) GreaterEqual(a, b *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 144); e != nil {
		return false, e
	}
	return m.manager.GreaterEqual(a, b)
}
func (m *IntegerModularManager) Swap(a, b *Int) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceAssertNormalizedPair(a, b, 188); e != nil {
		return e
	}
	return m.manager.Swap(a, b)
}
func (m *IntegerModularManager) SetInt32(a *Int, v int32) error {
	return m.setInt32WithSourceContext(a, v, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) setInt32WithSourceContext(a *Int, v int32, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.SetInt32(a, v); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(a, context)
}
func (m *IntegerModularManager) SetUint32(a *Int, v uint32) error {
	return m.setUint32WithSourceContext(a, v, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) setUint32WithSourceContext(a *Int, v uint32, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.setUint32WithAllocationContext(a, v, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(a, context)
}
func (m *IntegerModularManager) SetInt64(a *Int, v int64) error {
	return m.setInt64WithSourceContext(a, v, integerAllocationInvocationContext{})
}

func (m *IntegerModularManager) setInt64WithSourceContext(a *Int, v int64, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.setInt64WithAllocationContext(a, v, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(a, context)
}
func (m *IntegerModularManager) SetUint64(a *Int, v uint64) error {
	return m.setUint64WithSourceContext(a, v, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) setUint64WithSourceContext(a *Int, v uint64, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.setUint64WithAllocationContext(a, v, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(a, context)
}
func (m *IntegerModularManager) SetString(a *Int, v string) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.SetString(a, v); e != nil {
		return e
	}
	return m.PNormalize(a)
}
func (m *IntegerModularManager) Copy(a, b *Int) error {
	return m.copyWithSourceContext(a, b, integerAllocationInvocationContext{})
}

func (m *IntegerModularManager) copyWithSourceContext(a, b *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.copyFromOwnerWithAllocationContext(a, m.manager, b, nil, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(a, context)
}

// Exact target SET followed by source p_normalize, using the actual operand
// owner instead of a serializer or a temporary mirrored numeral backing.
func (m *IntegerModularManager) CopyFromOwner(a *Int, source *IntegerManager, b *Int) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.copyFromOwnerCore(a, source, b)
}
func (m *IntegerModularManager) copyFromOwnerCore(a *Int, source *IntegerManager, b *Int) error {
	return m.copyFromOwnerWithAllocationReceipt(a, source, b, nil)
}
func (m *IntegerModularManager) copyFromOwnerWithAllocationReceipt(a *Int, source *IntegerManager, b *Int, receipt *integerCopyAllocationReceipt) error {
	return m.copyFromOwnerWithAllocationReceipts(a, source, b, receipt, nil)
}
func (m *IntegerModularManager) copyFromOwnerWithAllocationReceipts(a *Int, source *IntegerManager, b *Int, receipt *integerCopyAllocationReceipt, pn *integerPNormalizeInvocationReceipt) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.CopyFromOwnerWithReceipt(a, source, b, receipt); e != nil {
		return e
	}
	return m.PNormalizeWithReceipt(a, pn)
}
func (m *IntegerModularManager) IsUint64(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	return m.manager.IsUint64(a)
}
func (m *IntegerModularManager) IsUint64Mutable(a *Int) (bool, error) {
	if e := m.PNormalize(a); e != nil {
		return false, e
	}
	return m.manager.IsUint64(a)
}
func (m *IntegerModularManager) IsInt64(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	return m.manager.IsInt64(a)
}
func (m *IntegerModularManager) IsInt64Mutable(a *Int) (bool, error) {
	if e := m.PNormalize(a); e != nil {
		return false, e
	}
	return m.manager.IsInt64(a)
}
func (m *IntegerModularManager) Uint64(a *Int) (uint64, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.manager.Uint64(a)
}
func (m *IntegerModularManager) Uint64Mutable(a *Int) (uint64, error) {
	if e := m.PNormalize(a); e != nil {
		return 0, e
	}
	return m.manager.Uint64(a)
}
func (m *IntegerModularManager) Int64(a *Int) (int64, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.manager.Int64(a)
}
func (m *IntegerModularManager) Int64Mutable(a *Int) (int64, error) {
	if e := m.PNormalize(a); e != nil {
		return 0, e
	}
	return m.manager.Int64(a)
}
func (m *IntegerModularManager) Float64Mutable(a *Int) (float64, error) {
	if e := m.PNormalize(a); e != nil {
		return 0, e
	}
	return m.manager.Float64(a)
}
func (m *IntegerModularManager) Hash(a *Int) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.manager.Hash(a)
}
func (m *IntegerModularManager) PowerOfTwoMultiple(a *Int) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.manager.PowerOfTwoMultiple(a)
}
func (m *IntegerModularManager) Log2(a *Int) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.manager.Log2(a)
}
func (m *IntegerModularManager) MLog2(a *Int) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.manager.MLog2(a)
}
func (m *IntegerModularManager) Mul2k(a *Int, k uint32, out *Int) error {
	return m.mul2kWithSourceContext(a, k, out, integerAllocationInvocationContext{})
}
func (m *IntegerModularManager) mul2kWithSourceContext(a *Int, k uint32, out *Int, context integerAllocationInvocationContext) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.shiftWithAllocationContext(a, k, out, true, context); e != nil {
		return e
	}
	return m.normalizeWithSourceContext(out, context)
}
func (m *IntegerModularManager) MachineDiv2k(a *Int, k uint32, out *Int) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.MachineDiv2k(a, k, out); e != nil {
		return e
	}
	return m.sourceAssertNormalizedNamed(out, 277, "is_p_normalized(r)")
}
func (m *IntegerModularManager) IsSmall(a *Int) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.manager.check(a); e != nil {
		return false, e
	}
	return a.IsSmall(), nil
}
func (m *IntegerModularManager) String(a *Int) (string, error) {
	if e := m.check(); e != nil {
		return "", e
	}
	if e := m.sourceAssertNormalized(a, 146); e != nil {
		return "", e
	}
	return m.manager.String(a)
}

// FormatInto is the source display path, which does not assert normalization.
func (m *IntegerModularManager) FormatInto(a *Int, dst []byte) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.manager.FormatInto(a, dst)
}
func (m *IntegerModularManager) Mul2kInPlace(a *Int, k uint32) error { return m.Mul2k(a, k, a) }
func (m *IntegerModularManager) MachineDiv2kInPlace(a *Int, k uint32) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.MachineDiv2kInPlace(a, k); e != nil {
		return e
	}
	return m.sourceAssertNormalized(a, 276)
}
