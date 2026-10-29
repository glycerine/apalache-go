package numeral

// Source src/util/mpff.h/.cpp.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// FastFloat owns a fixed significand block in its implicit manager. No owning
// shallow copies. Del retains dead scalar bits; Reset is a separate operation.
type FastFloat struct {
	bits     ffRef
	exponent int32
}

func NewFastFloat() FastFloat        { return FastFloat{} }
func (a *FastFloat) Exponent() int32 { return a.exponent }
func (a *FastFloat) index() uint32   { return uint32(a.bits) & 0x7fffffff }
func (a *FastFloat) negative() bool  { return uint32(a.bits)&0x80000000 != 0 }
func (a *FastFloat) sign(b bool) {
	if b {
		a.bits |= ffRef(1 << 31)
	} else {
		a.bits &^= ffRef(1 << 31)
	}
}

type FastFloatConfig struct {
	PrecisionWords, InitialCapacity uint32
	Storage                         arena.VectorConfig
	// Memory borrows the actual allocation runtime. Nil constructs an owned
	// runtime. Process binds that owned runtime to the source process counters.
	// When both are supplied they must refer to the same process.
	Memory  *umemory.Runtime
	Process *umemory.ProcessAccounting
}

func DefaultFastFloatConfig(c arena.VectorConfig) FastFloatConfig {
	return FastFloatConfig{PrecisionWords: 2, InitialCapacity: 1024, Storage: c}
}

type fastFloatException struct {
	message string
	parent  error
}

func (e *fastFloatException) Error() string                    { return e.message }
func (e *fastFloatException) Message() string                  { return e.message }
func (e *fastFloatException) ErrorCode() z3exception.ErrorCode { return z3exception.ErrorOK }
func (e *fastFloatException) HasErrorCode() bool               { return false }
func (e *fastFloatException) Unwrap() error                    { return e.parent }

var ErrFastFloatDomain error = &fastFloatException{message: "multi-precision floating point (mpff) exception"}
var ErrFastFloatOverflow error = &fastFloatException{message: "multi-precision floating point (mpff) overflow", parent: ErrFastFloatDomain}
var ErrFastFloatDivisionByZero error = &fastFloatException{message: "multi-precision floating point (mpff) division by zero", parent: ErrFastFloatDomain}
var ErrFastFloatInvariant = errors.New("mpnum: MPFF source invariant")
var ErrFastFloatPowerDomain = errors.New("mpnum: MPFF power mask wrapped into nontermination")
var ErrFastFloatPowerPending = errors.New("mpnum: MPFF power scoped cleanup pending")
var ErrFastFloatRationalPending = errors.New("mpnum: MPFF rational setter cleanup pending")
var ErrFastFloatIntegerPending = errors.New("mpnum: MPFF to-integer power cleanup pending")
var ErrFastFloatToRationalPending = errors.New("mpnum: MPFF to-rational power cleanup pending")
var ErrFastFloatFractionPending = errors.New("mpnum: MPFF fraction setter cleanup pending")
var ErrFastFloatOwner = errors.New("mpnum: invalid MPFF owner or dead significand")

// Cold owner: owns its buffers and services, borrowing a configured Runtime; caller numerical
// conversion managers are borrowed only for their individual operation.
type FastFloatManager struct {
	self                                         *FastFloatManager
	trace                                        *diagnostic.TraceTags
	natural                                      *NaturalManager
	config                                       FastFloatConfig
	precision, bits, capacity, next, live        uint32
	toPlus, closed                               bool
	headerBytes                                  uint64
	significands, metadata, freeIDs, setBuffer   []uint32
	buffers                                      [4][]uint32
	formatting                                   fastFormattingScope
	closing                                      bool
	one                                          FastFloat
	powerWork, powerTrace                        FastFloat
	powerWorkPending, powerTracePending          bool
	powerExecuting                               bool
	rationalNum, rationalDen                     FastFloat
	rationalPending, rationalExecuting           bool
	rationalCursor                               uint8
	integerPower                                 Int
	integerPowerOwner                            *IntegerManager
	integerPowerPending, integerExecuting        bool
	rationalPower                                Rational
	rationalPowerOwner                           *RationalManager
	rationalPowerPending, rationalPowerExecuting bool
	rationalPowerCursor                          uint8
	fractionNum, fractionDen                     FastFloat
	fractionPending, fractionExecuting           bool
	fractionCursor                               uint8
}

func NewFastFloatManager(c FastFloatConfig) (*FastFloatManager, error) {
	if c.PrecisionWords < 2 || c.PrecisionWords > uint32(0x7fffffff)/32 || c.InitialCapacity == 0 || uint64(c.InitialCapacity)*uint64(c.PrecisionWords) > uint64(^uint32(0))/4 || c.InitialCapacity > 0x7fffffff || c.Storage.Descriptors.Budget == nil {
		return nil, ErrFastFloatDomain
	}
	if c.Memory != nil {
		if e := c.Memory.CheckOpen(); e != nil {
			return nil, e
		}
		if c.Memory.Budget() != c.Storage.Descriptors.Budget {
			return nil, arena.ErrConfig
		}
	}
	if c.Process != nil && (!c.Process.Valid() || c.Memory != nil && c.Memory.ProcessOwner() != c.Process) {
		return nil, arena.ErrConfig
	}
	h := uint64(reflect.TypeOf(FastFloatManager{}).Size())
	if e := c.Storage.Descriptors.Budget.Reserve(h); e != nil {
		return nil, e
	}
	m := &FastFloatManager{config: c, precision: c.PrecisionWords, bits: c.PrecisionWords * 32, capacity: c.InitialCapacity, headerBytes: h}
	m.self = m
	var e error
	m.natural, e = newNaturalManagerWithProcess(c.Storage, c.Memory, c.Process)
	if e != nil {
		m.Close()
		return nil, e
	}
	if e = m.resize(&m.significands, m.capacity*m.precision); e == nil {
		e = m.resize(&m.metadata, m.capacity*2)
	}
	for i := 0; i < 4 && e == nil; i++ {
		e = m.resize(&m.buffers[i], 2*m.precision)
	}
	if e == nil {
		if m.mkID() != 0 {
			e = ErrFastFloatInvariant
		}
	}
	if e == nil {
		e = m.SetInt32(&m.one, 1)
	}
	if e != nil {
		m.Close()
		return nil, e
	}
	return m, nil
}

// RuntimeOwner returns the actual runtime identity without transferring ownership.
func (m *FastFloatManager) RuntimeOwner() *umemory.Runtime {
	if m == nil || m.self != m || m.natural == nil {
		return nil
	}
	return m.natural.RuntimeOwner()
}
func (m *FastFloatManager) open() error {
	if m == nil || m.self != m {
		return ErrFastFloatOwner
	}
	if m.closed || m.closing {
		return arena.ErrClosed
	}
	if (m.powerWorkPending || m.powerTracePending) && !m.powerExecuting {
		return ErrFastFloatPowerPending
	}
	if m.rationalPending && !m.rationalExecuting {
		return ErrFastFloatRationalPending
	}
	if m.integerPowerPending && !m.integerExecuting {
		return ErrFastFloatIntegerPending
	}
	if m.rationalPowerPending && !m.rationalPowerExecuting {
		return ErrFastFloatToRationalPending
	}
	if m.fractionPending && !m.fractionExecuting {
		return ErrFastFloatFractionPending
	}
	return nil
}
func (m *FastFloatManager) check(a *FastFloat) error {
	if e := m.open(); e != nil {
		return e
	}
	if a == nil {
		return arena.ErrInvalidSlot
	}
	i := a.index()
	if i == 0 {
		return nil
	}
	if i >= m.capacity || m.read(&m.metadata, 2*i) != 1 {
		return ErrFastFloatOwner
	}
	g := ffGeneration(a.bits)
	if g != 0 && g != m.read(&m.metadata, 2*i+1) {
		return ErrFastFloatOwner
	}
	return nil
}
func (m *FastFloatManager) pair(a, b *FastFloat) error {
	if e := m.check(a); e != nil {
		return e
	}
	return m.check(b)
}
func (m *FastFloatManager) triple(a, b, c *FastFloat) error {
	if e := m.pair(a, b); e != nil {
		return e
	}
	return m.check(c)
}
func (m *FastFloatManager) read(v *[]uint32, i uint32) uint32 { return (*v)[i] }
func (m *FastFloatManager) write(v *[]uint32, i, n uint32)    { (*v)[i] = n }
func (m *FastFloatManager) reserve(v *[]uint32, n uint32) error {
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(n) > cap(*v) {
		old := len(*v)
		*v = append(*v, make([]uint32, int(n)-old)...)
		*v = (*v)[:old]
	}
	return nil
}
func (m *FastFloatManager) resize(v *[]uint32, n uint32) error {
	if e := m.reserve(v, n); e != nil {
		return e
	}
	old := len(*v)
	if int(n) < old {
		clear((*v)[n:])
	}
	*v = (*v)[:n]
	if int(n) > old {
		clear((*v)[old:])
	}
	return nil
}
func (m *FastFloatManager) mkID() uint32 {
	if len(m.freeIDs) != 0 {
		i := m.read(&m.freeIDs, uint32(len(m.freeIDs))-1)
		m.resize(&m.freeIDs, uint32(len(m.freeIDs))-1)
		return i
	}
	i := m.next
	m.next++
	return i
}
func (m *FastFloatManager) allocate(a *FastFloat) error {
	if a.index() != 0 {
		return nil
	}
	i := m.mkID()
	if i == 0 || i > 0x7fffffff {
		return ErrFastFloatDomain
	}
	for i >= m.capacity {
		c := uint64(m.capacity) * 2
		if c > 0x7fffffff || c*uint64(m.precision) > uint64(^uint32(0))/4 {
			return arena.ErrBounds
		}
		if e := m.resize(&m.significands, uint32(c)*m.precision); e != nil {
			return e
		}
		if e := m.resize(&m.metadata, uint32(c)*2); e != nil {
			return e
		}
		m.capacity = uint32(c)
	}
	// Reserve native recycled-ID storage before publishing the new live block.
	needed := uint64(len(m.freeIDs)) + uint64(m.live) + 1
	if needed > uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	if e := m.reserve(&m.freeIDs, uint32(needed)); e != nil {
		return e
	}
	g := m.read(&m.metadata, 2*i+1)
	if g == ^uint32(0) {
		return ErrFastFloatOwner
	}
	g++
	m.write(&m.metadata, 2*i, 1)
	m.write(&m.metadata, 2*i+1, g)
	m.live++
	a.bits = ffPack(i, g, a.negative())
	if SourceExtendedGCDDebug && !ffIsZero(m.sig(a)) {
		return ErrFastFloatInvariant
	}
	return nil
}
func (m *FastFloatManager) sig(a *FastFloat) ffLimbView {
	return ffLimbView{m: m, v: &m.significands, offset: a.index() * m.precision, size: m.precision}
}
func (m *FastFloatManager) buffer(i int, size uint32) ffLimbView {
	return ffLimbView{m: m, v: &m.buffers[i], size: size}
}
func (m *FastFloatManager) Del(a *FastFloat) error {
	if e := m.check(a); e != nil {
		return e
	}
	i := a.index()
	if i == 0 {
		return nil
	}
	g := m.read(&m.metadata, 2*i+1)
	if g != ^uint32(0) {
		n := uint32(len(m.freeIDs))
		if e := m.resize(&m.freeIDs, n+1); e != nil {
			return e
		}
		m.write(&m.freeIDs, n, i)
		m.write(&m.metadata, 2*i, 0)
	} else {
		m.write(&m.metadata, 2*i, 2)
	}
	ffZero(m.sig(a))
	m.live--
	return nil
}
func (m *FastFloatManager) Reset(a *FastFloat) error {
	if e := m.Del(a); e != nil {
		return e
	}
	*a = FastFloat{}
	return m.debugCheck(a)
}
func (m *FastFloatManager) CloseChecked() error {
	if m == nil || m.self != m {
		return ErrFastFloatOwner
	}
	if m.closed {
		return nil
	}
	if m.powerExecuting {
		return ErrFastFloatPowerPending
	}
	if m.rationalExecuting {
		return ErrFastFloatRationalPending
	}
	if m.integerExecuting {
		return ErrFastFloatIntegerPending
	}
	if m.rationalPowerExecuting {
		return ErrFastFloatToRationalPending
	}
	if m.fractionExecuting {
		return ErrFastFloatFractionPending
	}
	m.closing = true
	if e := m.CleanupPowerTemporaries(); e != nil {
		return e
	}
	if e := m.CleanupSetRationalTemporaries(); e != nil {
		return e
	}
	if e := m.CleanupToIntegerPower(); e != nil {
		return e
	}
	if e := m.CleanupToRationalPower(); e != nil {
		return e
	}
	if e := m.CleanupFractionTemporaries(); e != nil {
		return e
	}
	if m.natural != nil {
		if e := m.CleanupFormatting(); e != nil {
			return e
		}
	}
	if m.natural != nil {
		if m.one.index() != 0 { // Del needs the intact owner during this final action.
			m.closing = false
			e := m.Del(&m.one)
			m.closing = true
			if e != nil {
				return e
			}
			m.one = FastFloat{}
		}
		if e := m.natural.CloseChecked(); e != nil {
			return e
		}
	}
	m.significands, m.metadata, m.freeIDs, m.setBuffer = nil, nil, nil, nil
	m.buffers = [4][]uint32{}
	m.config.Storage.Descriptors.Budget.Release(m.headerBytes)
	m.headerBytes = 0
	m.closed = true
	return nil
}
func (m *FastFloatManager) Close() {
	if m == nil || m.self != m {
		return
	}
	if e := m.CloseChecked(); e != nil {
		panic(m)
	}
}

func (m *FastFloatManager) cleanupPowerWork() error {
	if !m.powerWorkPending {
		return nil
	}
	if e := m.Del(&m.powerWork); e != nil {
		return e
	}
	m.powerWork = FastFloat{}
	m.powerWorkPending = false
	return nil
}

func (m *FastFloatManager) cleanupPowerTrace() error {
	if !m.powerTracePending {
		return nil
	}
	if e := m.Del(&m.powerTrace); e != nil {
		return e
	}
	m.powerTrace = FastFloat{}
	m.powerTracePending = false
	return nil
}

// CleanupPowerTemporaries resumes the scoped power work value before the
// outer trace snapshot, without replaying an already completed deletion.
func (m *FastFloatManager) CleanupPowerTemporaries() error {
	if m == nil || m.self != m {
		return ErrFastFloatOwner
	}
	if m.powerExecuting {
		return ErrFastFloatPowerPending
	}
	closing := m.closing
	m.closing = false
	m.powerExecuting = true
	defer func() {
		m.powerExecuting = false
		m.closing = closing
	}()
	if e := m.cleanupPowerWork(); e != nil {
		return e
	}
	return m.cleanupPowerTrace()
}

func (m *FastFloatManager) cleanupSetRationalTemporaries() error {
	if !m.rationalPending {
		return nil
	}
	roots := [2]*FastFloat{&m.rationalDen, &m.rationalNum}
	for m.rationalCursor < 2 {
		if e := m.Del(roots[m.rationalCursor]); e != nil {
			return e
		}
		*roots[m.rationalCursor] = FastFloat{}
		m.rationalCursor++
	}
	m.rationalPending = false
	m.rationalCursor = 0
	return nil
}

// CleanupSetRationalTemporaries resumes the scoped denominator then numerator.
func (m *FastFloatManager) CleanupSetRationalTemporaries() error {
	if m == nil || m.self != m {
		return ErrFastFloatOwner
	}
	if m.rationalExecuting {
		return ErrFastFloatRationalPending
	}
	closing := m.closing
	m.closing = false
	m.rationalExecuting = true
	defer func() {
		m.rationalExecuting = false
		m.closing = closing
	}()
	return m.cleanupSetRationalTemporaries()
}

func (m *FastFloatManager) cleanupToIntegerPower(source bool) error {
	if !m.integerPowerPending {
		return nil
	}
	var e error
	if source {
		_, e = sourceNumeralDel(m.integerPowerOwner, &m.integerPower)
	} else {
		e = m.integerPowerOwner.Del(&m.integerPower)
	}
	if e != nil {
		return e
	}
	m.integerPower = Int{}
	m.integerPowerOwner = nil
	m.integerPowerPending = false
	return nil
}

// CleanupToIntegerPower resumes the borrowed integer's scoped deletion.
func (m *FastFloatManager) CleanupToIntegerPower() error {
	if m == nil || m.self != m {
		return ErrFastFloatOwner
	}
	if m.integerExecuting {
		return ErrFastFloatIntegerPending
	}
	return m.cleanupToIntegerPower(false)
}

func (m *FastFloatManager) cleanupToRationalPower(source bool) error {
	if !m.rationalPowerPending {
		return nil
	}
	parts := [2]*Int{&m.rationalPower.Num, &m.rationalPower.Den}
	for m.rationalPowerCursor < 2 {
		var e error
		if source {
			_, e = sourceNumeralDel(m.rationalPowerOwner.Z, parts[m.rationalPowerCursor])
		} else {
			e = m.rationalPowerOwner.Z.Del(parts[m.rationalPowerCursor])
		}
		if e != nil {
			return e
		}
		m.rationalPowerCursor++
	}
	m.rationalPower = Rational{}
	m.rationalPowerOwner = nil
	m.rationalPowerPending = false
	m.rationalPowerCursor = 0
	return nil
}

// CleanupToRationalPower resumes numerator then denominator deletion.
func (m *FastFloatManager) CleanupToRationalPower() error {
	if m == nil || m.self != m {
		return ErrFastFloatOwner
	}
	if m.rationalPowerExecuting {
		return ErrFastFloatToRationalPending
	}
	return m.cleanupToRationalPower(false)
}

func (m *FastFloatManager) cleanupFractionTemporaries() error {
	if !m.fractionPending {
		return nil
	}
	parts := [2]*FastFloat{&m.fractionDen, &m.fractionNum}
	for m.fractionCursor < 2 {
		if e := m.Del(parts[m.fractionCursor]); e != nil {
			return e
		}
		*parts[m.fractionCursor] = FastFloat{}
		m.fractionCursor++
	}
	m.fractionPending = false
	m.fractionCursor = 0
	return nil
}

// CleanupFractionTemporaries resumes denominator then numerator deletion.
func (m *FastFloatManager) CleanupFractionTemporaries() error {
	if m == nil || m.self != m {
		return ErrFastFloatOwner
	}
	if m.fractionExecuting {
		return ErrFastFloatFractionPending
	}
	closing := m.closing
	m.closing = false
	m.fractionExecuting = true
	defer func() {
		m.fractionExecuting = false
		m.closing = closing
	}()
	return m.cleanupFractionTemporaries()
}

func (m *FastFloatManager) Precise() bool { return false }
func (m *FastFloatManager) Field() bool   { return true }
func (m *FastFloatManager) SetRounding(up bool) error {
	if e := m.open(); e != nil {
		return e
	}
	m.toPlus = up
	return nil
}
func (m *FastFloatManager) RoundToPlusInf() error  { return m.SetRounding(true) }
func (m *FastFloatManager) RoundToMinusInf() error { return m.SetRounding(false) }
func (m *FastFloatManager) RoundingToPlusInf() (bool, error) {
	if e := m.open(); e != nil {
		return false, e
	}
	return m.toPlus, nil
}
func (m *FastFloatManager) invariant(ok bool) error {
	if SourceExtendedGCDDebug && !ok {
		return ErrFastFloatInvariant
	}
	return nil
}
func (m *FastFloatManager) debugCheck(a *FastFloat) error {
	if !SourceExtendedGCDDebug {
		return nil
	}
	if a.index() == 0 {
		return m.invariant(!a.negative() && a.exponent == 0 && ffIsZero(m.sig(a)))
	}
	return m.invariant(m.sig(a).read(m.precision-1)&0x80000000 != 0)
}
func (m *FastFloatManager) Check(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if e := m.debugCheck(a); e != nil {
		return false, e
	}
	return true, nil
}
func (m *FastFloatManager) SetExponent(a *FastFloat, e int32) error {
	if x := m.check(a); x != nil {
		return x
	}
	if a.index() == 0 {
		return nil
	}
	a.exponent = e
	return m.debugCheck(a)
}
func (m *FastFloatManager) Copy(dst, src *FastFloat) error {
	if e := m.pair(dst, src); e != nil {
		return e
	}
	if src.index() == 0 {
		return m.Reset(dst)
	}
	if dst == src {
		return nil
	}
	if e := m.allocate(dst); e != nil {
		return e
	}
	dst.sign(src.negative())
	dst.exponent = src.exponent
	ffCopy(m.sig(src), m.sig(dst))
	return m.debugCheck(dst)
}
func (m *FastFloatManager) Swap(a, b *FastFloat) error {
	if e := m.pair(a, b); e != nil {
		return e
	}
	*a, *b = *b, *a
	return nil
}
func (m *FastFloatManager) Neg(a *FastFloat) error {
	if e := m.check(a); e != nil {
		return e
	}
	if a.index() != 0 {
		a.sign(!a.negative())
	}
	return nil
}
func (m *FastFloatManager) Abs(a *FastFloat) error {
	if e := m.check(a); e != nil {
		return e
	}
	a.sign(false)
	return nil
}
func (m *FastFloatManager) SetInt32(a *FastFloat, v int32) error {
	if e := m.check(a); e != nil {
		return e
	}
	n := uint32(v)
	if v < 0 {
		n = 0 - n
	}
	if e := m.SetUint32(a, n); e != nil {
		return e
	}
	if v < 0 {
		a.sign(true)
	}
	return m.debugCheck(a)
}
func (m *FastFloatManager) SetUint32(a *FastFloat, v uint32) error {
	if e := m.check(a); e != nil {
		return e
	}
	if v == 0 {
		return m.Reset(a)
	}
	if e := m.allocate(a); e != nil {
		return e
	}
	k := ffLeading(v)
	a.sign(false)
	a.exponent = 32 - int32(k) - int32(m.bits)
	s := m.sig(a)
	ffZero(s)
	s.write(m.precision-1, v<<k)
	return m.debugCheck(a)
}
func (m *FastFloatManager) SetInt64(a *FastFloat, v int64) error {
	if v == 0 {
		if e := m.Reset(a); e != nil {
			return e
		}
	} else {
		n := uint64(v)
		if v < 0 {
			n = 0 - n
		}
		if e := m.SetUint64(a, n); e != nil {
			return e
		}
		if v < 0 {
			a.sign(true)
		}
	}
	if e := m.debugCheck(a); e != nil {
		return e
	}
	if SourceExtendedGCDDebug {
		got, e := m.Int64(a)
		if e != nil {
			return e
		}
		return m.invariant(got == v)
	}
	return nil
}
func (m *FastFloatManager) SetUint64(a *FastFloat, v uint64) error {
	original := v
	if e := m.check(a); e != nil {
		return e
	}
	if v == 0 {
		if e := m.Reset(a); e != nil {
			return e
		}
	} else {
		if e := m.allocate(a); e != nil {
			return e
		}
		k := uint32(0)
		for v&(uint64(1)<<63) == 0 {
			v <<= 1
			k++
		}
		a.sign(false)
		a.exponent = 64 - int32(k) - int32(m.bits)
		s := m.sig(a)
		ffZero(s)
		s.write(m.precision-1, uint32(v>>32))
		s.write(m.precision-2, uint32(v))
	}
	if e := m.debugCheck(a); e != nil {
		return e
	}
	if SourceExtendedGCDDebug {
		got, e := m.Uint64(a)
		if e != nil {
			return e
		}
		return m.invariant(got == original)
	}
	return nil
}
func (m *FastFloatManager) SetFraction32(a *FastFloat, n int32, d uint32) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	m.fractionNum, m.fractionDen = FastFloat{}, FastFloat{}
	m.fractionPending = true
	m.fractionCursor = 0
	m.fractionExecuting = true
	defer func() {
		if cleanup := m.cleanupFractionTemporaries(); err == nil {
			err = cleanup
		}
		m.fractionExecuting = false
	}()
	if e := m.SetInt32(&m.fractionNum, n); e != nil {
		return e
	}
	if e := m.SetUint32(&m.fractionDen, d); e != nil {
		return e
	}
	return m.Div(&m.fractionNum, &m.fractionDen, a)
}
func (m *FastFloatManager) SetFraction64(a *FastFloat, n int64, d uint64) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	m.fractionNum, m.fractionDen = FastFloat{}, FastFloat{}
	m.fractionPending = true
	m.fractionCursor = 0
	m.fractionExecuting = true
	defer func() {
		if cleanup := m.cleanupFractionTemporaries(); err == nil {
			err = cleanup
		}
		m.fractionExecuting = false
	}()
	if e := m.SetInt64(&m.fractionNum, n); e != nil {
		return e
	}
	if e := m.SetUint64(&m.fractionDen, d); e != nil {
		return e
	}
	return m.Div(&m.fractionNum, &m.fractionDen, a)
}
