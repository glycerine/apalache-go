package numeral

// Source: src/util numeric primitive implementations.

type NumericRationalOperations struct {
	manager *RationalManager
	context NumericAllocationContext
}

func (m *RationalManager) WithAllocationContext(c NumericAllocationContext) NumericRationalOperations {
	return NumericRationalOperations{m, c}
}
func (o NumericRationalOperations) valid() error {
	if o.manager == nil {
		return ErrNumericOwner
	}
	return (NumericIntegerOperations{o.manager.Z, o.context}).valid()
}

type NumericBinaryRationalOperations struct {
	manager *BinaryRationalManager
	context NumericAllocationContext
}

func (m *BinaryRationalManager) WithAllocationContext(c NumericAllocationContext) NumericBinaryRationalOperations {
	return NumericBinaryRationalOperations{m, c}
}
func (o NumericBinaryRationalOperations) valid() error {
	if o.manager == nil {
		return ErrNumericOwner
	}
	return (NumericIntegerOperations{o.manager.Z, o.context}).valid()
}

type NumericBinaryIntervalOperations struct {
	manager *BinaryIntervalManager
	context NumericAllocationContext
}

func (m *BinaryIntervalManager) WithAllocationContext(c NumericAllocationContext) NumericBinaryIntervalOperations {
	return NumericBinaryIntervalOperations{m, c}
}
func (o NumericBinaryIntervalOperations) valid() error {
	if o.manager == nil {
		return ErrNumericOwner
	}
	return (NumericIntegerOperations{o.manager.bounds.Z, o.context}).valid()
}
func (o NumericIntegerOperations) GcdManyIndexed(as IntegerArrayView, g *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.gcdManyIndexedWithAllocationContext(as, g, o.context)
}
func (o NumericBinaryRationalOperations) Neg(a *BinaryRational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.negWithSourceContext(a, o.context)
}
func (o NumericBinaryIntervalOperations) Neg(a *BinaryInterval) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.negWithSourceContext(a, o.context)
}
func (o NumericRationalOperations) PrevPowerOfTwoWithRationalSetContext() (value uint32, cause error) {
	if e := o.valid(); e != nil {
		return 0, e
	}
	return o.manager.prevPowerOfTwoWithRationalSetContext(o.context)
}
func (o NumericRationalOperations) Neg(a *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.negWithSourceContext(a, o.context)
}
func (o NumericRationalOperations) Abs(a *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.absWithAllocationContext(a, o.context)
}
func (o NumericRationalOperations) Copy(dst, src *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.copyWithAllocationContext(dst, src, o.context)
}
func (o NumericRationalOperations) SetInt64(a *Rational, n int64) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setInt64WithAllocationContext(a, n, o.context)
}
func (o NumericRationalOperations) SetPartsFromIntegerOwner(a *Rational, n, d *Int, sourceOwner *IntegerManager) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setPartsFromIntegerOwnerWithAllocationContext(a, n, d, sourceOwner, o.context)
}
func (o NumericRationalOperations) Normalize(a *Rational) (_rawErr error) {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.normalizeWithAllocationContext(a, o.context)
}
func (o NumericRationalOperations) InvInPlace(a *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.invInPlaceWithAllocationContext(a, o.context)
}
func (o NumericRationalOperations) Compare(a, b *Rational) (_rawValue int, _rawErr error) {
	if e := o.valid(); e != nil {
		return 0, e
	}
	return o.manager.compareWithAllocationContext(a, b, o.context)
}
func (o NumericRationalOperations) AddSub(a, b, out *Rational, subtract bool) (_rawErr error) {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addSubWithAllocationContext(a, b, out, subtract, o.context)
}
func (o NumericRationalOperations) Mul(a, b, out *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.mulWithAllocationContext(a, b, out, o.context)
}
func (o NumericRationalOperations) SetInteger(a *Rational, n *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setIntegerWithAllocationContext(a, n, o.context)
}
func (o NumericRationalOperations) SetIntegerFromOwner(a *Rational, source *IntegerManager, n *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setIntegerFromOwnerWithAllocationContext(a, source, n, o.context)
}
func (o NumericRationalOperations) MulInt(a *Int, b, out *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.mulIntWithAllocationContext(a, b, out, o.context)
}
func (o NumericRationalOperations) DivInt(a *Rational, b *Int, out *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.divIntWithAllocationContext(a, b, out, o.context)
}
func (o NumericRationalOperations) Mod(a, b, out *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.modWithAllocationContext(a, b, out, o.context)
}
func (o NumericRationalOperations) RoundCore(a *Rational, out *Int, ceil bool) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.roundCoreWithAllocationContext(a, out, ceil, o.context)
}
func (o NumericRationalOperations) RoundToInt(a *Rational, out *Int, ceil bool) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.roundToIntWithAllocationContext(a, out, ceil, o.context)
}
func (o NumericIntegerOperations) AllocateIfNeeded(a *Int, capacity uint32) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.allocateIfNeededWithAllocationContext(a, capacity, o.context)
}
func (o NumericIntegerOperations) SetUint64(a *Int, v uint64) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setUint64WithAllocationContext(a, v, o.context)
}
func (o NumericIntegerOperations) CopyFromOwnerTracked(dst *Int, sourceManager *IntegerManager, src *Int, receipt *integerCopyAllocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.copyFromOwnerWithAllocationContext(dst, sourceManager, src, receipt, o.context)
}
func (o NumericIntegerOperations) Neg(a *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.negWithAllocationContext(a, o.context)
}
func (o NumericIntegerOperations) Abs(a *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.absWithAllocationContext(a, o.context)
}
func (o NumericIntegerOperations) EnsureTmp(idx int, size uint32) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.ensureTmpWithAllocationContext(idx, size, o.context)
}
func (o NumericIntegerOperations) PublishTmpAt(idx int, dst *Int, sign int32, size uint32) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.publishTmpAtWithAllocationContext(idx, dst, sign, size, o.context)
}
func (o NumericIntegerOperations) AddSub(a, b, c *Int, subtract bool) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addSubWithAllocationContext(a, b, c, subtract, o.context)
}
func (o NumericIntegerOperations) MachineDivRem(a, b, q, r *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.machineDivRemWithAllocationContext(a, b, q, r, o.context)
}
func (o NumericIntegerOperations) MachineDiv(a, b, c *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.machineDivWithAllocationContext(a, b, c, o.context)
}
func (o NumericIntegerOperations) Rem(a, b, c *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.remWithAllocationContext(a, b, c, o.context)
}
func (o NumericIntegerOperations) QuotRem(a, b, q, r *Int, mode int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.quotRemWithAllocationContext(a, b, q, r, mode, o.context)
}
func (o NumericIntegerOperations) DivideCells(a, b NumericCellRef) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.divideCellsWithAllocationContext(a, b, o.context)
}
func (o NumericIntegerOperations) ExtendedGCDDiagnostic(r1, r2, a, b, expected, nexta, nextb *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.extendedGCDDiagnosticWithAllocationContext(r1, r2, a, b, expected, nexta, nextb, o.context)
}
func (o NumericIntegerOperations) SetUint32(a *Int, n uint32) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setUint32WithAllocationContext(a, n, o.context)
}
func (o NumericIntegerOperations) AddMulTracked(a, b, c, d *Int, receipt *integerAddMulInvocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addMulWithAllocationContext(a, b, c, d, receipt, o.context)
}
func (o NumericIntegerOperations) AddSubMulTracked(a, b, c, d *Int, subtract bool, receipt *integerAddMulInvocationReceipt) (err error) {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addSubMulWithAllocationContext(a, b, c, d, subtract, receipt, o.context)
}
func (o NumericIntegerOperations) Gcd(a, b, c *Int) (err error) {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.gcdWithAllocationContext(a, b, c, o.context)
}
func (o NumericIntegerOperations) PowerRawCallWithContext(op uint8, a, b, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.powerRawCallWithContext(op, a, b, out, o.context)
}
func (o NumericIntegerOperations) EnsureOwnedCapacity(a *Int, capacity uint32) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.ensureOwnedCapacityWithAllocationContext(a, capacity, o.context)
}
func (o NumericIntegerOperations) PowerTracked(a *Int, p uint32, out *Int, receipt *integerPowerInvocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.powerWithAllocationContext(a, p, out, receipt, o.context)
}
func (o NumericIntegerOperations) PowerCoreWithContextTracked(a *Int, p uint32, out *Int, receipt *integerPowerInvocationReceipt) (cause error) {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.powerCoreWithContext(a, p, out, receipt, o.context)
}
func (o NumericIntegerOperations) Shift(a *Int, k uint32, out *Int, left bool) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.shiftWithAllocationContext(a, k, out, left, o.context)
}
func (o NumericIntegerOperations) Mul2kCore(a *Int, k uint32) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.mul2kCoreWithAllocationContext(a, k, o.context)
}
func (o NumericIntegerOperations) SquareCoreWithContext(a, out *Int) (exact bool, cause error) {
	if e := o.valid(); e != nil {
		return false, e
	}
	return o.manager.squareCoreWithContext(a, out, o.context)
}
func (o NumericIntegerOperations) BeginPerfectSquareRawScopeWithContext() error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.beginPerfectSquareRawScopeWithContext(o.context)
}
func (o NumericIntegerOperations) PerfectSquareCallWithContext(op uint8, a, b, out *Int) (cause error) {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.perfectSquareCallWithContext(op, a, b, out, o.context)
}
func (o NumericModularOperations) SetupP() error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setupPWithAllocationContext(o.context)
}
func (o NumericModularOperations) SetZP(p *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setZPWithAllocationContext(p, o.context)
}
func (o NumericModularOperations) PNormalizeWithFactorHistoryTracked(a *Int, receipt *integerPNormalizeInvocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.PNormalizeWithFactorHistory(a, receipt, o.context)
}
func (o NumericModularOperations) Inv(a *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.invWithAllocationContext(a, o.context)
}
func (o NumericModularOperations) Div(a, b, c *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.divWithAllocationContext(a, b, c, o.context)
}
func (o NumericModularOperations) Gcd(a, b, c *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.gcdWithSourceContext(a, b, c, o.context)
}
func (o NumericModularOperations) Neg(a *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.negWithSourceContext(a, o.context)
}
func (o NumericModularOperations) NegWithOwnersTracked(a *Int, receipt *integerPNormalizeInvocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.negWithOwners(a, receipt, o.context)
}
func (o NumericModularOperations) Abs(a *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.absWithSourceContext(a, o.context)
}
func (o NumericModularOperations) SubMul(a, b, c, d *Int, child *integerAddMulInvocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.subMulWithSourceContext(a, b, c, d, child, o.context)
}
func (o NumericModularOperations) SetInt32(a *Int, v int32) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setInt32WithSourceContext(a, v, o.context)
}
func (o NumericModularOperations) SetUint32(a *Int, v uint32) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setUint32WithSourceContext(a, v, o.context)
}
func (o NumericModularOperations) SetUint64(a *Int, v uint64) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setUint64WithSourceContext(a, v, o.context)
}
func (o NumericModularOperations) Mul2k(a *Int, k uint32, out *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.mul2kWithSourceContext(a, k, out, o.context)
}

func (o NumericModularOperations) AddMulTracked(a, b, c, d *Int, child *IntegerAddMulInvocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addMulWithSourceContext(a, b, c, d, child, o.context)
}
func (m *IntegerModularManager) CopyFromOwnerTracked(a *Int, source *IntegerManager, b *Int, copy *IntegerCopyAllocationReceipt, normalize *IntegerNormalizeInvocationReceipt) error {
	return m.copyFromOwnerWithAllocationReceipts(a, source, b, copy, normalize)
}
func (m *IntegerModularManager) NegWithReceipt(a *Int, r *IntegerNormalizeInvocationReceipt) error {
	return m.negWithPNReceipt(a, r)
}
func (m *IntegerModularManager) RootInPlaceWithReceipt(a *Int, n uint32, r *IntegerRootInvocationReceipt) (bool, error) {
	return m.rootInPlaceWithReceipt(a, n, r)
}

func (o NumericBinaryRationalOperations) AddSub(a, b, out *BinaryRational, subtract bool) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.addSubWithAllocationContext(a, b, out, subtract, o.context)
}
func (o NumericBinaryRationalOperations) Div2(a *BinaryRational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.div2WithAllocationContext(a, o.context)
}
func (o NumericBinaryRationalOperations) LessOneDiv2k(a *BinaryRational, k uint32) (bool, error) {
	if e := o.valid(); e != nil {
		return false, e
	}
	return o.manager.lessOneDiv2kWithAllocationContext(a, k, o.context)
}

func (o NumericBinaryRationalOperations) MulInteger(a *BinaryRational, b *Int, out *BinaryRational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.mulIntegerWithAllocationContext(a, b, out, o.context)
}

// FloorWithCopyAllocationReceipt preserves the numerator-copy receipt and carries
// caller allocation ownership through the later negative-value correction.
func (o NumericBinaryRationalOperations) FloorWithCopyAllocationReceipt(a *BinaryRational, out *Int, receipt *IntegerCopyAllocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.roundWithAllocationContext(a, o.manager.Z, out, false, receipt, o.context)
}

func (o NumericBinaryRationalOperations) LessRationalWithCopyAllocationReceipt(a *BinaryRational, qm *RationalManager, b *Rational, receipt *IntegerCopyAllocationReceipt) (bool, error) {
	if e := o.valid(); e != nil {
		return false, e
	}
	// Retained polynomial callers borrow this exact integer owner.
	if qm == nil || qm.Integers() != o.manager.Z {
		return false, ErrBinaryOwner
	}
	return o.manager.compareRationalWithAllocationContext(a, qm, b, 1, o.context, receipt)
}

func (o NumericBinaryRationalOperations) LessIntegerWithCopyAllocationReceipt(a *BinaryRational, b *Int, receipt *IntegerCopyAllocationReceipt) (bool, error) {
	if e := o.valid(); e != nil {
		return false, e
	}
	return o.manager.compareIntegerWithAllocationContext(a, b, false, o.context, receipt)
}

// PowerOfTwo is the retained mpbq power(mpbq(2), p, out) specialization.
// It preserves Copy then PowerInPlace, including exponent/assertion order.
func (o NumericBinaryRationalOperations) PowerOfTwo(p uint32, out *BinaryRational, receipt *IntegerCopyAllocationReceipt) error {
	if e := o.valid(); e != nil {
		return e
	}
	m := o.manager
	two := NewBinaryInt(2)
	if e := m.copyWithAllocationReceipt(out, &two, receipt); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	if e := m.debugAssert(uint64(out.KField)*uint64(p) <= uint64(^uint32(0))); e != nil {
		return e
	}
	out.KField *= p
	return m.Z.powerWithAllocationContext(&out.Num, p, &out.Num, nil, o.context)
}

func (o NumericRationalOperations) Div(a, b, out *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.divWithAllocationContext(a, b, out, o.context)
}
func (o NumericRationalOperations) SubMul(a, b, c, out *Rational) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.fusedWithAllocationContext(a, b, c, out, true, o.context)
}

// Dec carries the retained Berlekamp Q-I diagonal call through the same scalar
// decrement engine and subsequent modular normalization as the public method.
func (o NumericModularOperations) Dec(a *Int) error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.decWithSourceContext(a, o.context)
}

func (o NumericModularOperations) SetZ() error {
	if e := o.valid(); e != nil {
		return e
	}
	return o.manager.setZWithAllocationContext(o.context)
}
