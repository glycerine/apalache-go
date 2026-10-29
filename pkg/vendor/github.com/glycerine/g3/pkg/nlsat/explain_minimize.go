package nlsat

// Source: src/nlsat/nlsat_explain.cpp:1370-1437;
// src/nlsat/nlsat_scoped_literal_vector.h:35-47,66-69.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// explanationMinimizer is the retained explanation core-minimization subobject.
// It borrows the live registry through search, never a snapshot of its slices.
// Projection, normalization and the full explain functor are still separate.
type explanationMinimizer struct {
	self                *explanationMinimizer
	search              *searchState
	todo, core, newTodo []sat.Literal // borrowed source work vectors
	scopedLiteralVector
	refs                    [2]evaluatorIntervalRef
	union                   intervalUnionReceipt
	retired                 []*IntervalSet
	header                  uint64
	running, failed, closed bool
}

func newExplanationMinimizer(r *searchState) (*explanationMinimizer, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if r.minimizer != nil {
		return nil, arena.ErrConfig
	}
	h := uint64(reflect.TypeOf(explanationMinimizer{}).Size())
	if err := r.store.atoms.ineq.budget.Reserve(h); err != nil {
		return nil, err
	}
	m := &explanationMinimizer{search: r, header: h}
	m.self = m
	m.scopedLiteralVector.factory = r.store.atoms
	for i := range m.refs {
		m.refs[i].manager = r.ism
	}
	r.minimizer = m
	return m, nil
}
func (m *explanationMinimizer) owner() error {
	if m == nil || m.self != m {
		return ErrSearchOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if err := m.search.owner(); err != nil {
		return err
	}
	if m.search.minimizer != m {
		return ErrSearchOwner
	}
	return nil
}
func (m *explanationMinimizer) drop(i int) error {
	old, accepted := m.refs[i].value, m.refs[i].accepted
	if err := m.refs[i].reset(); err != nil {
		m.rememberRetired(old)
		m.rememberRetired(accepted)
		return err
	}
	return nil
}
func (m *explanationMinimizer) rememberRetired(s *IntervalSet) {
	if s == nil || !s.deleting && !s.closed {
		return
	}
	for _, old := range m.retired {
		if old == s {
			return
		}
	}
	m.retired = append(m.retired, s)
}
func (m *explanationMinimizer) clearIntervals() error {
	// Inner inf is destroyed before the outer union accumulator.
	for i := len(m.refs) - 1; i >= 0; i-- {
		if err := m.drop(i); err != nil {
			return err
		}
	}
	return nil
}
func (m *explanationMinimizer) addInfeasible(l sat.Literal, line int32) (bool, error) {
	r := m.search
	a, err := r.store.literalAtom(l)
	if err != nil {
		return false, err
	}
	if err = atomAssert("nlsat_explain.cpp", line, "a != 0", a != nil); err != nil {
		return false, err
	}
	inf, err := r.evaluator.InfeasibleIntervals(a, l.Sign())
	if err != nil {
		return false, err
	}
	// The evaluator's Go return transfers its already owned reference.
	m.refs[1].value = inf
	s, err := r.ism.mkUnionWithReceipt(inf, m.refs[0].value, &m.union)
	if err != nil {
		return false, err
	}
	old := m.refs[0].value
	if err = m.refs[0].assign(s); err != nil {
		m.rememberRetired(old)
		return false, err
	}
	m.union = intervalUnionReceipt{}
	full, err := r.ism.IsFull(m.refs[0].value)
	if err != nil {
		return false, err
	}
	if err = m.drop(1); err != nil {
		return false, err
	}
	return full, nil
}
func (m *explanationMinimizer) minimizeCore() (again bool, err error) {
	if err = atomAssert("nlsat_explain.cpp", 1373, "!todo.empty()", len(m.todo) > 0); err != nil {
		return false, err
	}
	m.newTodo = m.newTodo[:0]
	for _, l := range m.core {
		full, e := m.addInfeasible(l, 1383)
		if e != nil {
			return false, e
		}
		if full {
			return false, m.clearIntervals()
		}
	}
	if len(m.todo) == 1 {
		m.core = append(m.core, m.todo[0])
		return false, m.clearIntervals()
	}
	for _, l := range m.todo {
		full, e := m.addInfeasible(l, 1402)
		if e != nil {
			return false, e
		}
		if full {
			m.core = append(m.core, l)
			m.todo, m.newTodo = m.newTodo, m.todo
			return true, m.clearIntervals()
		}
		m.newTodo = append(m.newTodo, l)
	}
	return false, diagnostic.BorrowProcessDebugContext().UnreachableSource("src/nlsat/nlsat_explain.cpp", 1415)
}

// minimize appends an owning result, as source minimize appends to its caller's
// scoped vector. Input atoms must remain owned by the caller throughout the call.
// The returned slice borrows this component's result until resetResult or close.
func (m *explanationMinimizer) minimize(lits []sat.Literal) (result []sat.Literal, err error) {
	if err = m.owner(); err != nil {
		return nil, err
	}
	if err = m.search.open(); err != nil {
		return nil, err
	}
	m.running = true
	defer func() { m.running = false; m.failed = err != nil }()
	m.todo = append(m.todo[:0], lits...)
	m.core = m.core[:0]
	for {
		again, e := m.minimizeCore()
		if e != nil {
			return nil, e
		}
		if !again {
			break
		}
		for i, j := 0, len(m.todo)-1; i < j; i, j = i+1, j-1 {
			m.todo[i], m.todo[j] = m.todo[j], m.todo[i]
		}
		again, e = m.minimizeCore()
		if e != nil {
			return nil, e
		}
		if !again {
			break
		}
	}
	for _, l := range m.core {
		if err = m.scopedLiteralVector.push(l); err != nil {
			return nil, err
		}
	}
	return m.result, nil
}

func (m *explanationMinimizer) cleanup() error {
	if err := m.owner(); err != nil {
		return err
	}
	if m.running {
		return ErrSearchPending
	}
	ism := m.search.ism
	if err := ism.cleanupUnion(&m.union); err != nil {
		return err
	}
	for len(m.retired) > 0 {
		i := len(m.retired) - 1
		if err := ism.cleanupRetiredSet(m.retired[i]); err != nil {
			return err
		}
		m.retired = m.retired[:i]
	}
	if err := m.clearIntervals(); err != nil {
		return err
	}
	m.failed = false
	return nil
}

func (m *explanationMinimizer) resetResult() error {
	if err := m.owner(); err != nil {
		return err
	}
	if m.running {
		return ErrSearchPending
	}
	return m.scopedLiteralVector.reset()
}

func (m *explanationMinimizer) close() error {
	if m != nil && m.self == m && m.closed {
		return nil
	}
	if err := m.cleanup(); err != nil {
		return err
	}
	if err := m.resetResult(); err != nil {
		return err
	}
	m.search.minimizer = nil
	m.search.store.atoms.ineq.budget.Release(m.header)
	m.header = 0
	m.todo, m.core, m.newTodo, m.result = nil, nil, nil, nil
	m.closed = true
	return nil
}
