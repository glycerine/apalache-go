package numeral

// Source: src/util/mpf.cpp:61-67,1519-1569; src/util/mpf.h:65-73,241-259;
// src/util/map.h:285; src/util/hashtable.h:245-257.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/insdict"
)

// This is the actual MPF constructor/destructor dependency, distinct from the
// fixed-precision mpff implementation. Pointer values in powers2 are compressed
// manager-local mpz-wrapper IDs. No wrapper exists in default empty maps.
// MPF numeral algorithms and populated powers2 operations remain separate.
type ArbitraryFloatManager struct {
	formattedLiteral         arbitraryFloatFormattedLiteralScope
	signedFraction           arbitraryFloatSignedFractionScope
	hexfloatComponents       arbitraryFloatHexfloatComponentsScope
	binaryString             arbitraryFloatBinaryStringScope
	decimalStream            arbitraryFloatDecimalStreamScope
	smt2Stream               arbitraryFloatSMT2StreamScope
	rationalString           arbitraryFloatRationalStringScope
	ieeeBV                   arbitraryFloatIEEEBVScope
	sbv                      arbitraryFloatSBVScope
	toMPZ                    arbitraryFloatToMPZScope
	rem                      arbitraryFloatRemScope
	partialRemainder         arbitraryFloatPartialRemainderScope
	renormalize              arbitraryFloatRenormalizeScope
	sqrt                     arbitraryFloatSqrtScope
	fma                      arbitraryFloatFMAScope
	div                      arbitraryFloatDivScope
	mul                      arbitraryFloatMulScope
	addSub                   arbitraryFloatAddSubScope
	integralRound            arbitraryFloatIntegralRoundScope
	integerQuery             arbitraryFloatIntegerQueryScope
	plainRationalSet         arbitraryFloatPlainRationalSetScope
	RationalSetHistories     *arbitraryFloatRationalSetHistory
	rationalSetHistoryBytes  uint64
	rationalSet              arbitraryFloatRationalSetScope
	formattedSet             arbitraryFloatFormattedSetScope
	FormattedSetHistories    *arbitraryFloatFormattedSetHistory
	formattedSetHistoryBytes uint64

	roundHistories                                          *arbitraryFloatRoundHistory
	roundHistoryBytes                                       uint64
	RoundScope                                              ArbitraryFloatRoundScope
	RoundSqrtScope                                          ArbitraryFloatRoundScope
	inputBinding                                            arbitraryFloatInputBinding
	inputInvocation, inputHistoryBytes                      uint64
	InputCalls                                              *arbitraryFloatValueInputCall
	conversionHistory                                       *arbitraryFloatRationalConversionHistory
	conversionHistoryBytes                                  uint64
	rationalConversionInvocation                            uint64
	rationalConversion                                      arbitraryFloatRationalConversionScope
	FormatScopes                                            *ArbitraryFloatFormatInvocation
	formatInvocation                                        uint64
	PowerWrappers                                           *arbitraryFloatPowerWrapper
	powerNextID                                             uint32
	powerWrapperBytes                                       uint64
	self                                                    *ArbitraryFloatManager
	rational                                                *RationalManager
	integer                                                 *IntegerManager                 // source reference to the exact same mpq base
	powers                                                  [4]insdict.Dict[uint32, uint32] // p, pn, pm1, pm1n
	memory                                                  *umemory.Runtime
	trace                                                   *diagnostic.TraceTags
	budget                                                  *arena.Budget
	valueExecuting                                          bool
	valueCopy                                               integerCopyAllocationReceipt
	valueBinding                                            arbitraryFloatCopyBinding
	ValueCalls                                              *arbitraryFloatValueCopyCall
	valueInvocation, valueHistoryBytes                      uint64
	valueCopyAcknowledged                                   bool
	failedChild                                             error
	bodyCause, sourceCause, ownCause                        error
	header                                                  uint64
	rationalReady, constructorComplete, sourceEnded, closed bool
}

func NewArbitraryFloatManager(c IntegerConfig) (*ArbitraryFloatManager, error) {
	if c.Cells.Budget != c.Words.Descriptors.Budget {
		return nil, arena.ErrConfig
	}
	b := c.Cells.Budget
	bytes := uint64(reflect.TypeOf(ArbitraryFloatManager{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	m := &ArbitraryFloatManager{budget: b, header: bytes, memory: c.Memory}
	m.self = m
	q, e := NewRationalManager(RationalConfig{Integers: c, Synchronized: false})
	if e != nil {
		m.failedChild = e
		var child umemory.ConstructionFailure
		if errors.As(e, &child) {
			m.memory = child.RuntimeOwner()
		}
		m.bodyCause = e
		return nil, m
	}
	m.rational = q
	m.memory = q.RuntimeOwner()
	m.integer = q.Z
	q.mpfParent = m
	m.integer.MpfParent = m
	m.rationalReady = true
	m.constructorComplete = true
	return m, nil
}
func (m *ArbitraryFloatManager) Error() string { return "mpnum: retained arbitrary float manager" }
func (m *ArbitraryFloatManager) Unwrap() error {
	return errors.Join(m.bodyCause, m.sourceCause, m.ownCause)
}
func (m *ArbitraryFloatManager) OriginalCause() error { return m.bodyCause }
func (m *ArbitraryFloatManager) CleanupCause() error  { return m.ownCause }
func (m *ArbitraryFloatManager) RuntimeOwner() *umemory.Runtime {
	if m == nil || m.self != m {
		return nil
	}
	return m.memory
}
func (m *ArbitraryFloatManager) Cleanup() error { return m.Close() }
func (m *ArbitraryFloatManager) SourceFinalize() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.integer != nil && m.integer.stringSet.NumericExecuting() || m.formattedLiteral.executing || m.formattedLiteral.finishing || m.signedFraction.executing || m.signedFraction.finishing || m.hexfloatComponents.executing || m.hexfloatComponents.finishing || m.binaryString.executing || m.binaryString.finishing || m.decimalStream.executing || m.decimalStream.finishing || m.integer.decimalStreamExecuting(false) || m.smt2Stream.executing || m.smt2Stream.finishing || m.integer.smt2StreamExecuting(false) || m.rationalString.executing || m.rationalString.finishing || m.integer.rationalStringExecuting(false) || m.ieeeBV.executing || m.ieeeBV.finishing || m.sbv.executing || m.sbv.finishing || m.toMPZ.executing || m.toMPZ.finishing || m.integer.mpfToMPZExecuting(false) || m.rem.executing || m.rem.finishing || m.partialRemainder.executing || m.partialRemainder.finishing || m.renormalize.executing || m.renormalize.finishing || m.sqrt.executing || m.sqrt.finishing || m.fma.executing || m.fma.finishing || m.div.executing || m.div.finishing || m.mul.executing || m.mul.finishing || m.addSub.executing || m.addSub.finishing || m.integralRound.executing || m.integralRound.finishing || m.integerQuery.executing || m.integerQuery.finishing || m.plainRationalSet.executing || m.plainRationalSet.finishing || m.rationalSet.executing || m.rationalSet.finishing || (m.integer != nil && m.integer.rationalServiceReadCallbackActive()) || m.formattedSet.executing || m.formattedSet.finishing || m.rationalConversion.executing || m.rationalConversion.finishing || m.valueExecuting || m.formatInvocationExecuting() || (m.integer != nil && m.integer.mpfBorrowedSourceCallbackActive()) {
		return ErrNumericPending
	}
	// The caller-owned format context is a younger lexical scope. Query only
	// actual issued frames before touching cached class fields; ownership of
	// its unjoined leaves remains with the text caller.
	for f := m.FormatScopes; f != nil; f = f.Next {
		if !m.MatchesFormatFrame(f) {
			return umemory.ErrRuntimeOwner
		}
		if e := f.SourceReady(); e != nil && !f.owned {
			return e
		}
		if m.formatFrameUnfinished(f) {
			return ErrNumericPending
		}
	}
	if !m.constructorComplete {
		return arena.ErrClosed
	}
	if e := m.finishFormattedLiteralSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.integer.stringSetSourceReady(); e != nil {
		if m.integer.stringSet.sourceCause != nil && m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishSignedFractionSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishHexfloatComponentsSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishBinaryStringSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishDecimalStreamSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishSMT2StreamSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishRationalStringSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishIEEEBVSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishSBVSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishRemSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishPartialRemainderSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishSqrtSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishFMASource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishDivSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishMulSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishAddSubSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishIntegralRoundSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishIntegerQuerySource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishPlainRationalSetSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishRationalSetSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishFormattedSetSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishRoundSqrtSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if e := m.finishRoundSource(); e != nil {
		if m.sourceCause == nil {
			m.sourceCause = e
		}
		return e
	}
	if m.sourceCause != nil {
		return m.sourceCause
	}
	if m.sourceEnded {
		return nil
	}
	// All source NUMs and exact allocation children have already ended.
	// Retire only their now-empty cleanup protocol before the numerical
	// owner disappears; the caller's stream/result storage remains untouched.
	for f := m.FormatScopes; f != nil; f = f.Next {
		if !f.owned {
			f.owned = true
			f.active = false
			f.ownCursor = 5
		}
	}
	// The reached conversion is younger than this owner's cached values.
	// Completed source locals permit the class suffix even when exact unjoined
	// allocation roots still await separate owning acknowledgment.
	if m.rationalConversion.manager == m && m.rationalConversion.invocation != 0 {
		if e := m.finishRationalConversionSource(); e != nil {
			m.sourceCause = e
			return e
		}
	}
	if e := m.destroyPowerWrappers(false); e != nil {
		m.sourceCause = e
		return e
	}
	for i := 3; i >= 0; i-- {
		m.powers[i].Clear()
	}
	if e := m.rational.SourceFinalize(); e != nil {
		m.sourceCause = e
		return e
	}
	m.rationalReady = false
	m.sourceEnded = true
	return nil
}
func (m *ArbitraryFloatManager) Close() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.integer != nil && m.integer.stringSet.NumericExecuting() || m.formattedLiteral.executing || m.formattedLiteral.finishing || m.signedFraction.executing || m.signedFraction.finishing || m.hexfloatComponents.executing || m.hexfloatComponents.finishing || m.binaryString.executing || m.binaryString.finishing || m.decimalStream.executing || m.decimalStream.finishing || m.integer.decimalStreamExecuting(false) || m.smt2Stream.executing || m.smt2Stream.finishing || m.integer.smt2StreamExecuting(false) || m.rationalString.executing || m.rationalString.finishing || m.integer.rationalStringExecuting(false) || m.ieeeBV.executing || m.ieeeBV.finishing || m.sbv.executing || m.sbv.finishing || m.toMPZ.executing || m.toMPZ.finishing || m.integer.mpfToMPZExecuting(false) || m.rem.executing || m.rem.finishing || m.partialRemainder.executing || m.partialRemainder.finishing || m.renormalize.executing || m.renormalize.finishing || m.sqrt.executing || m.sqrt.finishing || m.fma.executing || m.fma.finishing || m.div.executing || m.div.finishing || m.mul.executing || m.mul.finishing || m.addSub.executing || m.addSub.finishing || m.integralRound.executing || m.integralRound.finishing || m.integerQuery.executing || m.integerQuery.finishing || m.plainRationalSet.executing || m.plainRationalSet.finishing || m.rationalSet.executing || m.rationalSet.finishing || (m.integer != nil && m.integer.rationalServiceReadCallbackActive()) || m.formattedSet.executing || m.formattedSet.finishing || m.rationalConversion.executing || m.rationalConversion.finishing || m.valueExecuting || m.formatInvocationExecuting() || (m.integer != nil && m.integer.mpfBorrowedSourceCallbackActive()) {
		return ErrNumericPending
	}
	// Registered diagnostic metadata is not ownership. Refuse only actual
	// unfinished lexical/owning children before retiring older class fields.
	for f := m.FormatScopes; f != nil; f = f.Next {
		if !m.MatchesFormatFrame(f) || m.formatFrameUnfinished(f) {
			return ErrNumericPending
		}
	}
	if m.closed {
		return nil
	}
	stop := func(e error) error {
		if e != nil && m.ownCause == nil {
			m.ownCause = e
		}
		return e
	}
	if e := m.CleanupFormattedLiteral(); e != nil {
		return stop(e)
	}
	if e := m.CleanupIntegerQuery(); e != nil {
		return stop(e)
	}
	if m.integer != nil {
		if e := m.integer.CleanupStringSet(); e != nil {
			return stop(e)
		}
	}
	if e := m.CleanupSignedFraction(); e != nil {
		return stop(e)
	}
	if e := m.CleanupHexfloatComponents(); e != nil {
		return stop(e)
	}
	if e := m.CleanupBinaryString(); e != nil {
		return stop(e)
	}
	if e := m.CleanupDecimalStream(); e != nil {
		return stop(e)
	}
	if e := m.CleanupSMT2Stream(); e != nil {
		return stop(e)
	}
	if e := m.CleanupRationalString(); e != nil {
		return stop(e)
	}
	if e := m.CleanupToIEEEBVMPZ(); e != nil {
		return stop(e)
	}
	if e := m.CleanupToSBVMPQ(); e != nil {
		return stop(e)
	}
	if e := m.CleanupToMPZ(); e != nil {
		return stop(e)
	}
	if e := m.CleanupRem(); e != nil {
		return stop(e)
	}
	if e := m.CleanupPartialRemainder(); e != nil {
		return stop(e)
	}
	if e := m.CleanupRenormalize(); e != nil {
		return stop(e)
	}
	if e := m.CleanupSqrt(); e != nil {
		return stop(e)
	}
	if e := m.CleanupFMA(); e != nil {
		return stop(e)
	}
	if e := m.CleanupDiv(); e != nil {
		return stop(e)
	}
	if e := m.CleanupMul(); e != nil {
		return stop(e)
	}
	if e := m.CleanupAddSub(); e != nil {
		return stop(e)
	}
	if e := m.CleanupRoundToIntegral(); e != nil {
		return stop(e)
	}
	if e := m.CleanupFormattedSet(); e != nil {
		return stop(e)
	}
	if e := m.CleanupRoundSqrt(); e != nil {
		return stop(e)
	}
	if e := m.CleanupPlainRationalSet(); e != nil {
		return stop(e)
	}
	if e := m.CleanupExponentRationalSet(); e != nil {
		return stop(e)
	}
	if e := m.CleanupRound(); e != nil {
		return stop(e)
	}
	// Retire the exact younger lexical conversion before older class members.
	// Its helper acknowledges completed children, owns local NUMs, then aborts
	// only the captured unjoined allocation children.
	if m.rationalConversion.manager == m && m.rationalConversion.invocation != 0 {
		if e := m.CleanupRationalConversion(); e != nil {
			return stop(e)
		}
	}
	if e := m.cleanupValueInputs(); e != nil {
		return stop(e)
	}
	if e := m.cleanupValueCopyCalls(); e != nil {
		return stop(e)
	}
	// A stopped value copy retains only the exact acquired child invocation.
	// Owning cleanup acknowledges it before destroying the older mpq member;
	// the immutable receipt is retained through the enclosing context end.
	if m.valueCopy.invocation != 0 && !m.valueCopyAcknowledged {
		if m.integer == nil || !m.integer.MatchesCopyAllocationInvocation(&m.valueCopy) {
			return stop(ErrNumericPending)
		}
		if e := m.integer.CleanupCopyAllocationInvocation(&m.valueCopy); e != nil {
			return stop(e)
		}
		m.valueCopyAcknowledged = true
	}
	if m.constructorComplete && !m.sourceEnded && m.sourceCause == nil {
		if e := m.SourceFinalize(); e != nil {
			return stop(e)
		}
	}
	if m.failedChild != nil {
		var child umemory.ConstructionFailure
		if errors.As(m.failedChild, &child) {
			if child.RuntimeOwner() != m.memory {
				return stop(umemory.ErrRuntimeOwner)
			}
			if e := child.Cleanup(); e != nil {
				return stop(e)
			}
		}
		m.failedChild = nil
	}
	if e := m.destroyPowerWrappers(true); e != nil {
		return stop(e)
	}
	for i := range m.powers {
		m.powers[i].Clear()
	}
	if m.rational != nil {
		if e := m.rational.CloseChecked(); e != nil {
			return stop(e)
		}
	}
	m.releasePowerWrapperHistory()
	m.releaseValueCopyHistory()
	m.releaseRationalSetHistories()
	m.releaseFormattedSetHistories()
	m.releaseRoundHistory()
	m.releaseValueInputHistory()
	m.releaseRationalConversionHistory()
	m.budget.Release(m.header)
	m.header = 0
	m.closed = true
	return nil
}

// OwnedStorageBytes reports this exact context's actual cold/payload storage.
// The borrowed Runtime is excluded; its physical block directory is charged by
// the shared Runtime owner once. This is accounting, not root acquisition.
func (m *ArbitraryFloatManager) OwnedStorageBytes() uint64 {
	if m == nil || m.self != m {
		panic(umemory.ErrRuntimeOwner)
	}
	n := m.binaryString.storageBytes() + m.rationalString.storageBytes() + m.header + m.rationalSetHistoryBytes + m.formattedSetHistoryBytes + m.roundHistoryBytes + m.valueHistoryBytes + m.powerWrapperBytes + m.conversionHistoryBytes + m.inputHistoryBytes
	var z *IntegerManager
	if m.rational != nil {
		n += m.rational.HeaderBytes + m.rational.stringScope.storageBytes()
		z = m.rational.Z
	} else {
		var failed *IntegerManager
		if errors.As(m.failedChild, &failed) {
			z = failed
		}
	}
	if z != nil {
		n += z.HeaderBytes
		if z.Allocator != nil {
			n += z.Allocator.Stats().TotalBytes
		}
		if z.Cells != nil {
			s := z.Cells.Stats()
			n += s.OwnerBytes + s.RecordBytes + s.MetadataBytes + s.DirectoryBytes
		}
		if z.Natural != nil {
			n += z.Natural.HeaderBytes
		}
	}
	return n
}

// formatFrameUnfinished reads only exact issued source locals and returned
// child receipts. Completed erased invocations and owning-acknowledged
// diagnostic records do not hold the older numerical manager alive.
func (m *ArbitraryFloatManager) formatFrameUnfinished(f *ArbitraryFloatFormatInvocation) bool {
	if f.owned {
		return false
	}
	if f.sourceCursor != 5 || NumericAllocationPending(&f.Numeric) || f.Transferred.Block() != (umemory.Block{}) {
		return true
	}
	if NumericAllocationPending(&f.Decimal.Numeric) || f.Decimal.Transferred.Block() != (umemory.Block{}) {
		return true
	}

	if f.Decimal.insertion.active && (!f.Decimal.insertion.sourceDone || f.Decimal.insertion.characterOwned || f.Decimal.insertion.formattingOwned) {
		return true
	}
	if f.Decimal.Parent != nil && !f.Decimal.ended && !f.Decimal.owned {
		return true
	}
	for _, r := range []*integerRawInvocationReceipt{&f.gcd, &f.division, &f.extended, &f.Decimal.division} {
		if m.integer.MatchesRawInvocation(r) && (!r.Frame.ended || r.Frame.pending(m.integer)) {
			return true
		}
	}
	return false
}
