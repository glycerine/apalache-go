package nlsat

// Source: src/nlsat/nlsat_interval_set.cpp/.h.

import (
	"errors"
	"io"
	"math"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

var ErrIntervalOwner = errors.New("nlsat: invalid interval-set owner")
var ErrIntervalLive = errors.New("nlsat: live interval sets")

type infeasibleInterval struct {
	lowerOpen, upperOpen, lowerInf, upperInf bool
	justification                            sat.Literal
	lower, upper                             polynomial.Algebraic
}

// IntervalSet is an opaque, reference-counted set. Nil denotes the empty set.
// Newly allocated sets have zero references, exactly as in the source factory.
type IntervalSet struct {
	self                        *IntervalSet
	owner                       *IntervalSetManager
	intervals                   []infeasibleInterval
	refs                        uint32
	full                        bool
	raw                         umemory.ObjectRef
	extent                      uint64
	release                     umemory.ReleaseProgress
	releasing, deleting, closed bool
}

// IntervalSetManager borrows the algebraic manager and allocator. Both outlive
// it, and callers retire their sets before closing this manager.
type IntervalSetManager struct {
	self             *IntervalSetManager
	search           *searchState
	am               *polynomial.AlgebraicManager
	allocator        *umemory.SmallAllocator
	budget           *arena.Budget
	rand             z3rand.Generator
	visited          []bool
	live             map[*IntervalSet]struct{}
	retired          []*IntervalSet
	pending          []*intervalBuffer
	justifications   *intervalJustificationScope
	allocation       umemory.SmallAllocatorAllocationOutcome
	allocationFailed bool
	header           uint64
	closing, closed  bool
}

func NewIntervalSetManager(am *polynomial.AlgebraicManager, a *umemory.SmallAllocator) (*IntervalSetManager, error) {
	if am == nil || a == nil {
		return nil, arena.ErrConfig
	}
	q, err := am.RationalManager()
	if err != nil {
		return nil, err
	}
	if a.RuntimeOwner() != q.Integers().RuntimeOwner() {
		return nil, arena.ErrConfig
	}
	if _, err = a.NumFreeObjects(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(IntervalSetManager{}).Size())
	if err = q.Budget().Reserve(n); err != nil {
		return nil, err
	}
	m := &IntervalSetManager{am: am, allocator: a, budget: q.Budget(), header: n, live: map[*IntervalSet]struct{}{}}
	m.self = m
	return m, nil
}
func (m *IntervalSetManager) owner() error {
	if m == nil || m.self != m || m.closed {
		return ErrIntervalOwner
	}
	return nil
}
func (m *IntervalSetManager) open() error {
	if err := m.owner(); err != nil {
		return err
	}
	if m.closing || m.allocationFailed || len(m.retired) > 0 || len(m.pending) > 0 || m.justifications != nil {
		return polynomial.ErrAlgebraicPending
	}
	_, err := m.am.RationalManager()
	return err
}
func (m *IntervalSetManager) checkSet(s *IntervalSet) error {
	if s == nil {
		return nil
	}
	if s.self != s || s.owner != m || s.closed || s.deleting {
		return ErrIntervalOwner
	}
	if _, ok := m.live[s]; !ok {
		return ErrIntervalOwner
	}
	return nil
}
func (m *IntervalSetManager) SetSeed(seed uint32) error {
	if err := m.open(); err != nil {
		return err
	}
	m.rand.SetSeed(seed)
	return nil
}
func (*IntervalSetManager) Empty() *IntervalSet { return nil }
func (m *IntervalSetManager) IsEmpty(s *IntervalSet) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if err := m.checkSet(s); err != nil {
		return false, err
	}
	return s == nil, nil
}
func (m *IntervalSetManager) IsFull(s *IntervalSet) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if err := m.checkSet(s); err != nil {
		return false, err
	}
	return s != nil && s.full, nil
}
func (m *IntervalSetManager) NumIntervals(s *IntervalSet) (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	if err := m.checkSet(s); err != nil {
		return 0, err
	}
	if s == nil {
		return 0, nil
	}
	return uint32(len(s.intervals)), nil
}
func (m *IntervalSetManager) allocate(n int) (*IntervalSet, error) {
	return m.allocateForBuffer(n, nil)
}
func (m *IntervalSetManager) allocateForBuffer(n int, b *intervalBuffer) (*IntervalSet, error) {
	if n < 1 || uint64(n) > math.MaxUint32 {
		return nil, arena.ErrBounds
	}
	extent := uint64(reflect.TypeOf(IntervalSet{}).Size()) + uint64(n)*uint64(reflect.TypeOf(infeasibleInterval{}).Size())
	raw, started, err := m.allocator.AllocateWithOutcomeStarted(extent, &m.allocation)
	if b != nil && started {
		b.allocation, b.allocationFailed = m.allocation, err != nil
	}
	if err != nil {
		m.allocationFailed = started
		return nil, err
	}
	s := &IntervalSet{owner: m, intervals: make([]infeasibleInterval, n), raw: raw, extent: extent}
	s.self = s
	m.live[s] = struct{}{}
	return s, nil
}
func (m *IntervalSetManager) deleteSet(s *IntervalSet) error {
	if s.closed {
		return nil
	}
	s.deleting = true
	for i := range s.intervals {
		if err := m.am.Del(&s.intervals[i].lower); err != nil {
			return err
		}
		if err := m.am.Del(&s.intervals[i].upper); err != nil {
			return err
		}
	}
	var err error
	if s.releasing {
		err = m.allocator.CompleteRelease(s.extent, s.raw, &s.release)
	} else {
		s.release, err = m.allocator.DeallocateTracked(s.extent, s.raw)
		s.releasing = s.release != (umemory.ReleaseProgress{})
	}
	if err != nil {
		return err
	}
	s.raw = umemory.ObjectRef{}
	s.intervals = nil
	s.closed = true
	delete(m.live, s)
	return nil
}
func (m *IntervalSetManager) Cleanup() error {
	if err := m.owner(); err != nil {
		return err
	}
	if err := m.cleanupJustifications(m.justifications); err != nil {
		return err
	}
	for len(m.pending) > 0 {
		b := m.pending[len(m.pending)-1]
		if err := b.close(); err != nil {
			return err
		}
		m.pending = m.pending[:len(m.pending)-1]
	}
	for len(m.retired) > 0 {
		s := m.retired[len(m.retired)-1]
		if err := m.deleteSet(s); err != nil {
			return err
		}
		m.retired = m.retired[:len(m.retired)-1]
	}
	if m.allocationFailed {
		if m.allocator.MatchesAllocationOutcome(&m.allocation) {
			if err := m.allocator.CleanupPending(); err != nil {
				return err
			}
		}
		m.allocationFailed = false
	}
	return nil
}
func (m *IntervalSetManager) Close() error {
	if m != nil && m.self == m && m.closed {
		return nil
	}
	if m != nil && m.self == m && m.search != nil {
		return ErrSearchLive
	}
	if err := m.owner(); err != nil {
		return err
	}
	if err := m.Cleanup(); err != nil {
		return err
	}
	if len(m.live) != 0 {
		return ErrIntervalLive
	}
	m.closing = true
	m.budget.Release(uint64(cap(m.visited)))
	m.visited = nil
	m.budget.Release(m.header)
	m.header = 0
	m.closed = true
	return nil
}
func (m *IntervalSetManager) IncRef(s *IntervalSet) error {
	if err := m.open(); err != nil {
		return err
	}
	if err := m.checkSet(s); err != nil {
		return err
	}
	if s != nil {
		s.refs = (s.refs + 1) & 0x7fffffff
	}
	return nil
}
func (m *IntervalSetManager) DecRef(s *IntervalSet) error {
	if err := m.open(); err != nil {
		return err
	}
	if err := m.checkSet(s); err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	if err := intervalAssert(138, "s->m_ref_count > 0", s.refs > 0); err != nil {
		return err
	}
	if s.refs == 0 {
		return arena.ErrBounds
	}
	s.refs--
	if s.refs == 0 {
		if err := m.deleteSet(s); err != nil {
			m.retired = append(m.retired, s)
			return err
		}
	}
	return nil
}
func intervalAssert(line int32, text string, ok bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/nlsat/nlsat_interval_set.cpp", line, text)
}
func (m *IntervalSetManager) checkIntervals(ints []infeasibleInterval) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	for i := range ints {
		curr := &ints[i]
		if curr.lowerInf {
			if err = intervalAssert(74, "i.m_lower_open", curr.lowerOpen); err != nil {
				return err
			}
		}
		if curr.upperInf {
			if err = intervalAssert(77, "i.m_upper_open", curr.upperOpen); err != nil {
				return err
			}
		}
		if !curr.lowerInf && !curr.upperInf {
			sign, e := m.am.Compare(&curr.lower, &curr.upper)
			if e != nil {
				return e
			}
			if err = intervalAssert(83, "s <= 0", sign <= 0); err != nil {
				return err
			}
			if sign == 0 {
				if err = intervalAssert(85, "!i.m_lower_open && !i.m_upper_open", !curr.lowerOpen && !curr.upperOpen); err != nil {
					return err
				}
			}
		}
		if i+1 < len(ints) {
			next := &ints[i+1]
			if err = intervalAssert(92, "!curr.m_upper_inf", !curr.upperInf); err != nil {
				return err
			}
			if err = intervalAssert(93, "!next.m_lower_inf", !next.lowerInf); err != nil {
				return err
			}
			sign, e := m.am.Compare(&curr.upper, &next.lower)
			if e != nil {
				return e
			}
			if err = intervalAssert(96, "sign <= 0", sign <= 0); err != nil {
				return err
			}
			if sign == 0 {
				if err = intervalAssert(98, "curr.m_upper_open || next.m_lower_open", curr.upperOpen || next.lowerOpen); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func (m *IntervalSetManager) Mk(lowerOpen, lowerInf bool, lower *polynomial.Algebraic, upperOpen, upperInf bool, upper *polynomial.Algebraic, justification sat.Literal) (s *IntervalSet, err error) {
	if err = m.open(); err != nil {
		return nil, err
	}
	if !lowerInf && lower == nil || !upperInf && upper == nil {
		return nil, arena.ErrConfig
	}
	s, err = m.allocate(1)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			m.retired = append(m.retired, s)
			s = nil
		}
	}()
	s.full = lowerInf && upperInf
	i := &s.intervals[0]
	i.lowerOpen = lowerOpen
	i.lowerInf = lowerInf
	i.upperOpen = upperOpen
	i.upperInf = upperInf
	i.justification = justification
	if !lowerInf {
		if err = m.am.Copy(&i.lower, lower); err != nil {
			return s, err
		}
	}
	if !upperInf {
		if err = m.am.Copy(&i.upper, upper); err != nil {
			return s, err
		}
	}
	err = m.checkIntervals(s.intervals)
	return s, err
}

// Each buffer record has a stable address while numerical copy calls run.
// Publishing the final array moves these owners without copying their values.
type intervalBuffer struct {
	m                  *IntervalSetManager
	entries            []*infeasibleInterval
	bytes              uint64
	allocation         umemory.SmallAllocatorAllocationOutcome
	allocationFailed   bool
	published, retired *IntervalSet
}

func (b *intervalBuffer) push(i *infeasibleInterval) error {
	n := uint64(reflect.TypeOf(infeasibleInterval{}).Size()) + uint64(reflect.TypeOf((*infeasibleInterval)(nil)).Size())
	if err := b.m.budget.Reserve(n); err != nil {
		return err
	}
	b.bytes += n
	v := &infeasibleInterval{justification: sat.DefaultLiteral()}
	b.entries = append(b.entries, v)
	v.lowerOpen, v.lowerInf = i.lowerOpen, i.lowerInf
	if err := b.m.am.Copy(&v.lower, &i.lower); err != nil {
		return err
	}
	v.upperOpen, v.upperInf = i.upperOpen, i.upperInf
	if err := b.m.am.Copy(&v.upper, &i.upper); err != nil {
		return err
	}
	v.justification = i.justification
	return nil
}
func (b *intervalBuffer) close() error {
	for _, i := range b.entries {
		if err := b.m.am.Del(&i.lower); err != nil {
			return err
		}
		if err := b.m.am.Del(&i.upper); err != nil {
			return err
		}
	}
	b.entries = nil
	b.m.budget.Release(b.bytes)
	b.bytes = 0
	return nil
}
func (m *IntervalSetManager) finishBuffer(b *intervalBuffer, err *error) {
	cleanup := b.close()
	if cleanup != nil {
		m.pending = append(m.pending, b)
	}
	*err = errors.Join(*err, cleanup)
}
func (m *IntervalSetManager) publish(b *intervalBuffer, full bool) (*IntervalSet, error) {
	s, err := m.allocateForBuffer(len(b.entries), b)
	if err != nil {
		return nil, err
	}
	b.published = s
	s.full = full
	for j, i := range b.entries {
		s.intervals[j] = *i
		i.lower = polynomial.Algebraic{}
		i.upper = polynomial.Algebraic{}
	}
	if err = m.checkIntervals(s.intervals); err != nil {
		m.retired = append(m.retired, s)
		b.retired = s
		return nil, err
	}
	return s, nil
}
func (m *IntervalSetManager) GetInterval(s *IntervalSet, index uint32) (result *IntervalSet, err error) {
	if err = m.open(); err != nil {
		return nil, err
	}
	if err = m.checkSet(s); err != nil {
		return nil, err
	}
	if s == nil || uint64(index) >= uint64(len(s.intervals)) {
		return nil, arena.ErrBounds
	}
	b := &intervalBuffer{m: m}
	defer m.finishBuffer(b, &err)
	if err = b.push(&s.intervals[index]); err != nil {
		return nil, err
	}
	return m.publish(b, b.entries[0].lowerInf && b.entries[0].upperInf)
}
func (m *IntervalSetManager) GetJustifications(s *IntervalSet, out *[]sat.Literal) error {
	return m.getJustificationsWithReceipt(s, out, nil)
}
func (m *IntervalSetManager) getJustificationsWithReceipt(s *IntervalSet, out *[]sat.Literal, receipt **intervalJustificationScope) (err error) {
	if err := m.open(); err != nil {
		return err
	}
	if err := m.checkSet(s); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	*out = (*out)[:0]
	if s == nil {
		return nil
	}
	scope := &intervalJustificationScope{owner: m}
	m.justifications = scope
	if receipt != nil {
		*receipt = scope
	}
	defer func() {
		if err == nil {
			m.justifications = nil
			scope.marked = nil
		}
	}()
	for _, i := range s.intervals {
		idx := uint64(i.justification.Index())
		if idx < uint64(len(m.visited)) && m.visited[idx] {
			continue
		}
		if idx >= uint64(len(m.visited)) {
			if idx >= uint64(^uint(0)>>1) {
				return arena.ErrBounds
			}
			if err := m.budget.Reserve(idx + 1); err != nil {
				return err
			}
			next := make([]bool, int(idx+1))
			copy(next, m.visited)
			m.budget.Release(uint64(cap(m.visited)))
			m.visited = next
		}
		m.visited[idx] = true
		scope.marked = append(scope.marked, uint32(idx))
		*out = append(*out, i.justification)
	}
	for _, i := range s.intervals {
		m.visited[i.justification.Index()] = false
	}
	return nil
}
func (m *IntervalSetManager) Display(out io.Writer, s *IntervalSet) error {
	if err := m.open(); err != nil {
		return err
	}
	if err := m.checkSet(s); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	w := &assignmentWriter{out: out}
	if s == nil {
		_, _ = io.WriteString(w, "{}")
		return w.err
	}
	_, _ = io.WriteString(w, "{")
	for j := range s.intervals {
		if j > 0 {
			_, _ = io.WriteString(w, ", ")
		}
		i := &s.intervals[j]
		if i.lowerInf {
			_, _ = io.WriteString(w, "(-oo, ")
		} else {
			left := "["
			if i.lowerOpen {
				left = "("
			}
			_, _ = io.WriteString(w, left)
			if err := m.am.DisplayDecimal(w, &i.lower, 10); err != nil {
				return errors.Join(err, w.err)
			}
			_, _ = io.WriteString(w, ", ")
		}
		if i.justification.Sign() {
			_, _ = io.WriteString(w, "~")
		}
		_, _ = io.WriteString(w, "p"+strconv.FormatUint(uint64(i.justification.Variable()), 10)+", ")
		if i.upperInf {
			_, _ = io.WriteString(w, "oo)")
		} else {
			if err := m.am.DisplayDecimal(w, &i.upper, 10); err != nil {
				return errors.Join(err, w.err)
			}
			right := "]"
			if i.upperOpen {
				right = ")"
			}
			_, _ = io.WriteString(w, right)
		}
	}
	_, _ = io.WriteString(w, "}")
	if s.full {
		_, _ = io.WriteString(w, "*")
	}
	return w.err
}
