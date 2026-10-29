package polynomial

// Flat polynomial sequence with native row-offset and row-size metadata.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolySequenceState struct {
	coefficients  upolyVectorState
	begins, sizes []uint32
}
type UPolynomialSequence struct {
	sturm                upolySequenceSturmScope
	evaluation           upolyBinaryEvaluationInvocationReceipt
	evaluationSourceStop error
	evaluating           bool
	callerBinding        unumeral.NumericCallerBinding
	self                 *UPolynomialSequence
	manager              *UPolynomialManager
	state                upolySequenceState
	header               uint64
	closed               bool
	displaying           bool
}
type UPolynomialSequenceStats struct {
	OwnerBytes, MetadataBytes, ManagerBackingBytes, TotalBytes                                                   uint64
	BeginCount, SizeCount, CoefficientCount, BeginCapacity, SizeCapacity, CoefficientCapacity, OwnedNumeralCells uint32
}

// Append metadata through native storage, retaining the public uint32 domain.
func upolyUnsignedPush(s *[]uint32, v uint32) error {
	if uint64(len(*s)) >= math.MaxUint32 || len(*s) == int(^uint(0)>>1) {
		return arena.ErrBounds
	}
	*s = append(*s, v)
	return nil
}
func (m *UPolynomialManager) NewSequence() (*UPolynomialSequence, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	if m.liveSequences == math.MaxUint32 {
		return nil, arena.ErrSlotLimit
	}
	n := uint64(reflect.TypeOf(UPolynomialSequence{}).Size())
	if e := m.config.Integer.Budget().Reserve(n); e != nil {
		return nil, e
	}
	s := &UPolynomialSequence{manager: m, header: n}
	s.self = s
	m.liveSequences++
	return s, nil
}
func (s *UPolynomialSequence) check() error {
	if s == nil || s.self != s || s.manager == nil {
		return ErrUPolynomialOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return s.manager.check()
}
func (m *UPolynomialManager) sequencePush(s *upolySequenceState, v UPolynomialView, move bool) error {
	if e := upolyUnsignedPush(&s.begins, s.coefficients.size); e != nil {
		return e
	}
	if e := upolyUnsignedPush(&s.sizes, v.count); e != nil {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		destination, e := m.pushDefault(&s.coefficients)
		if e != nil {
			return e
		}
		source, e := m.read(v, i)
		if e != nil {
			return e
		}
		if move {
			*destination, *source = *source, *destination
		} else if e = m.modular.Copy(destination, source); e != nil {
			return e
		}
	}
	return nil
}
func (s *UPolynomialSequence) PushCopy(v UPolynomialView) error {
	if s.mutationPending() {
		return ErrUPolynomialCleanupPending
	}
	if e := s.check(); e != nil {
		return e
	}
	return s.manager.sequencePush(&s.state, v, false)
}
func (s *UPolynomialSequence) PushMove(v UPolynomialView) error {
	if s.mutationPending() {
		return ErrUPolynomialCleanupPending
	}
	if e := s.check(); e != nil {
		return e
	}
	return s.manager.sequencePush(&s.state, v, true)
}
func (s *UPolynomialSequence) Size() (uint32, error) {
	if e := s.check(); e != nil {
		return 0, e
	}
	return uint32(len(s.state.sizes)), nil
}
func upolyUnsignedRead(s []uint32, i uint32) (uint32, error) {
	if polynomialSourceDebug && uint64(i) >= uint64(len(s)) {
		return 0, ErrUPolynomialAssertion
	}
	if uint64(i) >= uint64(len(s)) {
		return 0, arena.ErrBounds
	}
	return s[i], nil
}
func (s *UPolynomialSequence) RowSize(i uint32) (uint32, error) {
	if e := s.check(); e != nil {
		return 0, e
	}
	return upolyUnsignedRead(s.state.sizes, i)
}
func (m *UPolynomialManager) sequenceRow(s *upolySequenceState, i uint32) (UPolynomialView, error) {
	begin, e := upolyUnsignedRead(s.begins, i)
	if e != nil {
		return UPolynomialView{}, e
	}
	n, e := upolyUnsignedRead(s.sizes, i)
	if e != nil {
		return UPolynomialView{}, e
	}
	v := m.stateView(&s.coefficients)
	v.start = begin
	v.count = n
	return v, nil
}
func (s *UPolynomialSequence) RowView(i uint32) (UPolynomialView, error) {
	if e := s.check(); e != nil {
		return UPolynomialView{}, e
	}
	return s.manager.sequenceRow(&s.state, i)
}
func (m *UPolynomialManager) sequenceReset(s *upolySequenceState) error {
	if e := m.reset(&s.coefficients); e != nil {
		return e
	}
	s.sizes = s.sizes[:0]
	s.begins = s.begins[:0]
	return nil
}
func (s *UPolynomialSequence) Reset() error {
	if s.mutationPending() {
		return ErrUPolynomialCleanupPending
	}
	if e := s.check(); e != nil {
		return e
	}
	return s.manager.sequenceReset(&s.state)
}
func (m *UPolynomialManager) sequenceDestroy(s *upolySequenceState) error {
	if e := m.sequenceReset(s); e != nil {
		return e
	}
	s.sizes = nil
	s.begins = nil
	m.freeBacking(&s.coefficients)
	return nil
}
func (s *UPolynomialSequence) Close() error {
	if s != nil && s.self == s && (s.displaying || s.evaluating || s.sturm.executing || s.sturm.finishing) {
		return ErrUPolynomialCleanupPending
	}
	if e := s.check(); e != nil {
		return e
	}
	if e := s.CleanupEvaluation(); e != nil {
		return e
	}
	if e := s.CleanupSturm(); e != nil {
		return e
	}
	if e := s.manager.sequenceDestroy(&s.state); e != nil {
		return e
	}
	s.manager.liveSequences--
	s.manager.config.Integer.Budget().Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}

// Coefficients and row metadata use native slices. TotalBytes counts only the
// distinct cold owner; MetadataBytes and ManagerBackingBytes stay zero.
func (s *UPolynomialSequence) Stats() UPolynomialSequenceStats {
	if s == nil || s.self != s || s.closed || s.manager.check() != nil {
		return UPolynomialSequenceStats{}
	}
	a := &s.state
	r := UPolynomialSequenceStats{OwnerBytes: s.header, BeginCount: uint32(len(a.begins)), SizeCount: uint32(len(a.sizes)), CoefficientCount: a.coefficients.size, BeginCapacity: uint32(cap(a.begins)), SizeCapacity: uint32(cap(a.sizes)), CoefficientCapacity: uint32(cap(s.manager.values(&a.coefficients)))}
	r.TotalBytes = r.OwnerBytes + r.MetadataBytes
	for i := uint32(0); i < a.coefficients.size; i++ {
		if s.manager.config.Integer.IsLiveCellToken(s.manager.values(&a.coefficients)[i].NativeCellToken()) {
			r.OwnedNumeralCells++
		}
	}
	return r
}
