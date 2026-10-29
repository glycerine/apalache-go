package nlsat

// Concrete non-value-refcounting dependency_manager<solver::imp::dconfig>.
// Source: src/util/dependency.h:28-221; src/nlsat/nlsat_solver.cpp:56-64.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrAssumptionOwner = errors.New("nlsat: invalid assumption-set owner")
var ErrAssumptionPending = errors.New("nlsat: pending assumption operation")
var ErrAssumptionLive = errors.New("nlsat: live assumption sets")

const assumptionMaxRefs uint32 = (1 << 30) - 1

// AssumptionSet is an opaque dependency DAG node. New factories return zero
// references; joins acquire references to their children, never to leaf values.
type AssumptionSet struct {
	self                           *AssumptionSet
	owner                          *assumptionManager
	refs                           uint32
	marked, leaf, closed, deleting bool
	value                          Assumption
	children                       [2]*AssumptionSet
	raw                            umemory.ObjectRef
	extent                         uint64
	release                        umemory.ReleaseProgress
	releasing                      bool
}
type assumptionManager struct {
	self                             *assumptionManager
	allocator                        *umemory.SmallAllocator
	budget                           *arena.Budget
	live                             map[*AssumptionSet]struct{}
	todo                             []*AssumptionSet
	visitRoot                        *AssumptionSet
	deleteRoot, current, childToPush *AssumptionSet
	childIndex                       uint8
	mode                             uint8 // 0 idle, 1 traversal, 2 deletion
	allocation                       umemory.SmallAllocatorAllocationOutcome
	allocationFailed                 bool
	header                           uint64
	deleteSerial, activeDelete       uint64
	clauses                          *clauseStore
	executing, closed                bool
}

type assumptionDeletionReceipt struct {
	owner  *assumptionManager
	serial uint64
}

type assumptionLeafReceipt struct {
	owner      *assumptionManager
	allocation umemory.SmallAllocatorAllocationOutcome
	failed     bool
	leaf       *AssumptionSet
}

func newAssumptionManager(a *umemory.SmallAllocator) (*assumptionManager, error) {
	if a == nil || a.RuntimeOwner() == nil {
		return nil, arena.ErrConfig
	}
	if _, err := a.NumFreeObjects(); err != nil {
		return nil, err
	}
	b := a.RuntimeOwner().Budget()
	n := uint64(reflect.TypeOf(assumptionManager{}).Size())
	if err := b.Reserve(n); err != nil {
		return nil, err
	}
	m := &assumptionManager{allocator: a, budget: b, header: n, live: map[*AssumptionSet]struct{}{}}
	m.self = m
	return m, nil
}
func (m *assumptionManager) owner() error {
	if m == nil || m.self != m {
		return ErrAssumptionOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	return m.allocator.RuntimeOwner().CheckOpen()
}
func (m *assumptionManager) open() error {
	if err := m.owner(); err != nil {
		return err
	}
	if m.executing || m.mode != 0 || m.allocationFailed {
		return ErrAssumptionPending
	}
	return nil
}
func (m *assumptionManager) check(d *AssumptionSet) error {
	if d == nil {
		return nil
	}
	if d.self != d || d.owner != m || d.closed || d.deleting {
		return ErrAssumptionOwner
	}
	if _, ok := m.live[d]; !ok {
		return ErrAssumptionOwner
	}
	return nil
}
func assumptionAssert(line int32, text string, ok bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/util/dependency.h", line, text)
}
func (m *assumptionManager) allocate(leaf bool) (*AssumptionSet, error) {
	return m.allocateWithLeafReceipt(leaf, nil)
}
func (m *assumptionManager) allocateWithLeafReceipt(leaf bool, receipt *assumptionLeafReceipt) (*AssumptionSet, error) {
	previous := m.executing
	m.executing = true
	defer func() { m.executing = previous }()
	n := uint64(reflect.TypeOf(AssumptionSet{}).Size())
	raw, started, err := m.allocator.AllocateWithOutcomeStarted(n, &m.allocation)
	if receipt != nil && started {
		*receipt = assumptionLeafReceipt{owner: m, allocation: m.allocation, failed: err != nil}
	}
	if err != nil {
		m.allocationFailed = started
		return nil, err
	}
	d := &AssumptionSet{owner: m, leaf: leaf, raw: raw, extent: n}
	d.self = d
	m.live[d] = struct{}{}
	if receipt != nil {
		receipt.owner, receipt.leaf = m, d
	}
	return d, nil
}
func (m *assumptionManager) mkLeaf(v Assumption) (*AssumptionSet, error) {
	return m.mkLeafWithReceipt(v, nil)
}
func (m *assumptionManager) mkLeafWithReceipt(v Assumption, receipt *assumptionLeafReceipt) (*AssumptionSet, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	d, err := m.allocateWithLeafReceipt(true, receipt)
	if err != nil {
		return nil, err
	}
	d.value = v
	return d, nil
}

func (m *assumptionManager) cleanupLeaf(receipt *assumptionLeafReceipt) error {
	if err := m.owner(); err != nil {
		return err
	}
	if receipt == nil || receipt.owner != m {
		return nil
	}
	if receipt.failed && m.allocator.MatchesAllocationOutcome(&receipt.allocation) {
		if err := m.allocator.CleanupPending(); err != nil {
			return err
		}
		m.allocationFailed = false
		receipt.failed = false
	}
	if d := receipt.leaf; d != nil && !d.closed {
		if err := m.open(); err != nil {
			return err
		}
		if err := m.check(d); err != nil {
			return err
		}
		if !d.leaf || d.refs != 0 {
			return ErrAssumptionLive
		}
		// This concrete leaf was created with zero references and never
		// acquired by the failed clause. Retire it directly, without an
		// invented INC/DEC pair or touching another deletion invocation.
		m.executing = true
		err := m.free(d)
		m.executing = false
		if err != nil {
			return err
		}
	}
	*receipt = assumptionLeafReceipt{}
	return nil
}
func (m *assumptionManager) mkJoin(a, b *AssumptionSet) (*AssumptionSet, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if err := m.check(a); err != nil {
		return nil, err
	}
	if err := m.check(b); err != nil {
		return nil, err
	}
	if a == nil {
		return b, nil
	}
	if b == nil || a == b {
		return a, nil
	}
	d, err := m.allocate(false)
	if err != nil {
		return nil, err
	}
	a.refs = (a.refs + 1) & assumptionMaxRefs
	b.refs = (b.refs + 1) & assumptionMaxRefs
	d.children = [2]*AssumptionSet{a, b}
	return d, nil
}
func (m *assumptionManager) incRef(d *AssumptionSet) error {
	if err := m.open(); err != nil {
		return err
	}
	if err := m.check(d); err != nil {
		return err
	}
	if d != nil {
		d.refs = (d.refs + 1) & assumptionMaxRefs
	}
	return nil
}
func (m *assumptionManager) decRef(d *AssumptionSet) error {
	return m.decRefWithReceipt(d, nil)
}
func (m *assumptionManager) decRefWithReceipt(d *AssumptionSet, receipt *assumptionDeletionReceipt) error {
	if err := m.open(); err != nil {
		return err
	}
	if err := m.check(d); err != nil {
		return err
	}
	if d == nil {
		return nil
	}
	if err := assumptionAssert(133, "d->m_ref_count > 0", d.refs > 0); err != nil {
		return err
	}
	if d.refs == 0 {
		return arena.ErrBounds
	}
	if d.refs == 1 && m.deleteSerial == ^uint64(0) {
		return arena.ErrBounds
	}
	d.refs--
	if d.refs != 0 {
		return nil
	}
	m.mode = 2
	m.deleteSerial++
	m.activeDelete = m.deleteSerial
	if receipt != nil {
		*receipt = assumptionDeletionReceipt{m, m.activeDelete}
	}
	m.deleteRoot = d
	d.deleting = true
	m.executing = true
	defer func() { m.executing = false }()
	return m.drainDelete()
}
func (m *assumptionManager) push(d *AssumptionSet) error {
	if len(m.todo) == cap(m.todo) {
		n := cap(m.todo) * 2
		if n == 0 {
			n = 4
		}
		if n < cap(m.todo) {
			return arena.ErrBounds
		}
		size := uint64(reflect.TypeOf((*AssumptionSet)(nil)).Size())
		if err := m.budget.Reserve(uint64(n) * size); err != nil {
			return err
		}
		next := make([]*AssumptionSet, len(m.todo), n)
		copy(next, m.todo)
		m.budget.Release(uint64(cap(m.todo)) * size)
		m.todo = next
	}
	m.todo = append(m.todo, d)
	return nil
}
func (m *assumptionManager) free(d *AssumptionSet) error {
	var err error
	if d.releasing {
		err = m.allocator.CompleteRelease(d.extent, d.raw, &d.release)
	} else {
		d.release, err = m.allocator.DeallocateTracked(d.extent, d.raw)
		d.releasing = d.release != (umemory.ReleaseProgress{})
	}
	if err != nil {
		return err
	}
	d.raw = umemory.ObjectRef{}
	d.children = [2]*AssumptionSet{}
	d.value = Assumption{}
	d.closed = true
	delete(m.live, d)
	return nil
}
func (m *assumptionManager) drainDelete() error {
	if m.deleteRoot != nil {
		if err := m.push(m.deleteRoot); err != nil {
			return err
		}
		m.deleteRoot = nil
	}
	for m.current != nil || len(m.todo) > 0 {
		if m.current == nil {
			i := len(m.todo) - 1
			m.current = m.todo[i]
			m.todo[i] = nil
			m.todo = m.todo[:i]
			m.current.deleting = true
			m.childIndex = 0
		}
		d := m.current
		if !d.leaf {
			for m.childIndex < 2 {
				if m.childToPush != nil {
					if err := m.push(m.childToPush); err != nil {
						return err
					}
					m.childToPush = nil
					m.childIndex++
					continue
				}
				c := d.children[m.childIndex]
				if err := assumptionAssert(99, "c->m_ref_count > 0", c.refs > 0); err != nil {
					return err
				}
				if c.refs == 0 {
					return arena.ErrBounds
				}
				c.refs--
				if c.refs == 0 {
					c.deleting = true
					m.childToPush = c
				} else {
					m.childIndex++
				}
			}
		}
		if err := m.free(d); err != nil {
			return err
		}
		m.current = nil
	}
	m.mode = 0
	m.activeDelete = 0
	return nil
}

func (m *assumptionManager) cleanupDeletion(receipt *assumptionDeletionReceipt) error {
	if err := m.owner(); err != nil {
		return err
	}
	if receipt != nil && receipt.owner == m && receipt.serial == m.activeDelete && m.mode == 2 {
		return m.cleanup()
	}
	return nil
}
func (m *assumptionManager) unmark() {
	if m.visitRoot != nil {
		m.visitRoot.marked = false
	}
	for _, d := range m.todo {
		d.marked = false
	}
	clear(m.todo)
	m.todo = m.todo[:0]
	m.visitRoot = nil
	m.mode = 0
}
func (m *assumptionManager) traverse(d *AssumptionSet, v Assumption, out *[]Assumption) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if err := m.check(d); err != nil {
		return false, err
	}
	if d == nil {
		return false, nil
	}
	m.executing = true
	defer func() { m.executing = false }()
	m.todo = m.todo[:0]
	m.mode = 1
	m.visitRoot = d
	d.marked = true
	if err := m.push(d); err != nil {
		return false, err
	}
	for head := 0; head < len(m.todo); head++ {
		d = m.todo[head]
		if d.leaf {
			if out != nil {
				*out = append(*out, d.value)
			} else if d.value == v {
				m.unmark()
				return true, nil
			}
		} else {
			for _, c := range d.children {
				if !c.marked {
					if err := m.push(c); err != nil {
						return false, err
					}
					c.marked = true
				}
			}
		}
	}
	m.unmark()
	return false, nil
}
func (m *assumptionManager) contains(d *AssumptionSet, v Assumption) (bool, error) {
	return m.traverse(d, v, nil)
}
func (m *assumptionManager) linearize(d *AssumptionSet, out *[]Assumption) error {
	if out == nil {
		return arena.ErrConfig
	}
	_, err := m.traverse(d, Assumption{}, out)
	return err
}
func (m *assumptionManager) cleanup() error {
	if err := m.owner(); err != nil {
		return err
	}
	if m.executing {
		return ErrAssumptionPending
	}
	if m.mode == 1 {
		m.unmark()
	}
	if m.mode == 2 {
		m.executing = true
		err := m.drainDelete()
		m.executing = false
		if err != nil {
			return err
		}
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
func (m *assumptionManager) close() error {
	if m != nil && m.self == m && m.closed {
		return nil
	}
	if m != nil && m.self == m && m.clauses != nil {
		if m.clauses.running {
			return ErrAssumptionPending
		}
		return ErrAssumptionLive
	}
	if err := m.cleanup(); err != nil {
		return err
	}
	if len(m.live) != 0 {
		return ErrAssumptionLive
	}
	m.budget.Release(uint64(cap(m.todo)) * uint64(reflect.TypeOf((*AssumptionSet)(nil)).Size()))
	m.todo = nil
	m.budget.Release(m.header)
	m.header = 0
	m.closed = true
	return nil
}
