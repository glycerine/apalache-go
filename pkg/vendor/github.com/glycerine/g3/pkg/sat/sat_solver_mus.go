package sat

// Source: src/sat/sat_mus.cpp:28–229; src/sat/sat_mus.h; src/sat/sat_solver.cpp:1257–1261,2072–2083.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrSATMUSEmptyRecursion = errors.New("sat: empty MUS qx recursion has no finite source continuation")

// A dynamic lexical frame names actual source local value owners. Its reflected
// cold header is a Go ownership adapter, not an extra source vector allocation.
// Borrowed qx arguments never own the same backing twice. Failed source bodies
// retain interrupted destructors here, without restoring outputs or marks.
type satMUSFrame struct {
	appendActive       bool
	appendOld          uint32
	innerCoreCompleted bool
	next               *satMUSFrame
	sets               [3]satMUSLiteralSet
	vector             satUintColumn
	setsUsed           uint32
	vectorUsed         bool
	drain              uint32
	cleanupStopped     bool
	bytes              uint64
}

func (m *satSolverMUSMembers) frame(sets uint32, vector bool) (*satMUSFrame, error) {
	s := m.solver
	n := uint64(reflect.TypeOf(satMUSFrame{}).Size())
	if e := s.runtime.budget.Reserve(n); e != nil {
		return nil, e
	}
	f := &satMUSFrame{next: m.frames, setsUsed: sets, vectorUsed: vector, bytes: n}
	f.vector.Bind(s.runtime.memory)
	m.frames = f
	return f, nil
}
func (m *satSolverMUSMembers) drop(f *satMUSFrame, cause error) error {
	if f.cleanupStopped {
		return cause
	}
	if m.frames != f {
		if cause != nil {
			return cause
		}
		return ErrSATOwner
	}
	// Source locals unwind in reverse declaration order, interrupted at the
	// actual reached release. A failed constructor's buffers are owning-aborted
	// only after the older completed locals; current constructors are empty.
	if f.vectorUsed {
		if e := f.vector.Finalize(); e != nil {
			return errors.Join(cause, e)
		}
		f.vectorUsed = false
	}
	if f.innerCoreCompleted {
		f.sets[0].close()
		f.innerCoreCompleted = false
		f.setsUsed = 0
	}
	if f.appendActive {
		if e := satMUSResize(&m.mus, f.appendOld); e != nil {
			return errors.Join(cause, e)
		}
		f.appendActive = false
	}
	for f.drain < f.setsUsed {
		i := f.setsUsed - 1 - f.drain
		f.sets[i].close()
		f.drain++
	}
	if m.frames != f {
		return errors.Join(cause, ErrSATOwner)
	}
	m.frames = f.next
	m.solver.runtime.budget.Release(f.bytes)
	return cause
}
func (m *satSolverMUSMembers) cleanup() error {
	for m.frames != nil {
		m.frames.cleanupStopped = false
		if e := m.drop(m.frames, nil); e != nil {
			return e
		}
	}
	return nil
}
func satMUSAppend(dst, src *satUintColumn) error {
	for i := uint32(0); ; i++ {
		n, e := src.Size()
		if e != nil {
			return e
		}
		if i >= n {
			return nil
		}
		v, e := src.Read(i)
		if e != nil {
			return e
		}
		if e = dst.Push(v); e != nil {
			return e
		}
	}
}
func satMUSContains(v *satUintColumn, l uint32) (bool, error) {
	for i := uint32(0); ; i++ {
		n, e := v.Size()
		if e != nil {
			return false, e
		}
		if i >= n {
			return false, nil
		}
		a, e := v.Read(i)
		if e != nil {
			return false, e
		}
		if a == l {
			return true, nil
		}
	}
}
func (m *satSolverMUSMembers) reset() error {
	if e := m.core.Shrink(0); e != nil {
		return e
	}
	if e := m.mus.Shrink(0); e != nil {
		return e
	}
	return m.model.Shrink(0)
}
func (m *satSolverMUSMembers) setCore() error {
	if e := satMUSAppend(&m.mus, &m.core); e != nil {
		return e
	}
	if e := m.solver.conflict.core.Shrink(0); e != nil {
		return e
	}
	if e := satMUSAppend(&m.solver.conflict.core, &m.mus); e != nil {
		return e
	}
	m.traceMUSCore(40, "new core: ", &m.solver.conflict.core)
	return nil
}
func (m *satSolverMUSMembers) updateModel() error {
	n, e := m.model.Size()
	if e != nil {
		return e
	}
	if n == 0 {
		return satMUSAppend(&m.model, &m.solver.model)
	}
	return nil
}
func (m *satSolverMUSMembers) getCore() error {
	if e := m.core.Shrink(0); e != nil {
		return e
	}
	if e := m.mus.Shrink(0); e != nil {
		return e
	}
	if e := satMUSAppend(&m.core, &m.solver.conflict.core); e != nil {
		return e
	}
	for i := uint32(0); ; i++ {
		n, e := m.core.Size()
		if e != nil {
			return e
		}
		if i >= n {
			return nil
		}
		v, e := m.core.Read(i)
		if e != nil {
			return e
		}
		yes, e := satMUSContains(&m.solver.conflict.userScopeLiterals, v)
		if e != nil {
			return e
		}
		if yes {
			v, e = m.core.Read(i)
			if e != nil {
				return e
			}
			if e = m.mus.Push(v); e != nil {
				return e
			}
			n, e = m.core.Size()
			if e != nil {
				return e
			}
			v, e = m.core.Read(n - 1)
			if e != nil {
				return e
			}
			if e = m.core.Write(i, v); e != nil {
				return e
			}
			if e = m.core.Shrink(n - 1); e != nil {
				return e
			}
			i--
		}
	}
}
func (m *satSolverMUSMembers) run() (lbool.LBool, error) {
	if m == nil || m.solver == nil || m != &m.solver.mus {
		return lbool.Undef, ErrSATOwner
	}
	if e := m.solver.open(); e != nil {
		return lbool.Undef, e
	}
	if m.frames != nil || m.isActive {
		return lbool.Undef, arena.ErrClosed
	}
	cfg := &m.solver.config.state
	if cfg.CoreMinimizePartial {
		m.maxNumRestarts = m.solver.gc.restarts + 10
	} else {
		m.maxNumRestarts = ^uint32(0)
	}
	oldMin := cfg.CoreMinimize
	cfg.CoreMinimize = false
	oldActive := m.isActive
	m.isActive = true
	defer func() { m.isActive = oldActive; cfg.CoreMinimize = oldMin }()
	if e := m.emitMUSEntry(); e != nil {
		return lbool.Undef, e
	}
	if e := m.reset(); e != nil {
		return lbool.Undef, e
	}
	return m.mus1()
}
func (m *satSolverMUSMembers) scopedCheck(core *satUintColumn, lit Literal) (r lbool.LBool, e error) {
	cfg := &m.solver.config.state
	oldRestart := cfg.RestartMax
	cfg.RestartMax = m.maxNumRestarts
	defer func() { cfg.RestartMax = oldRestart }()
	n, e := m.mus.Size()
	if e != nil {
		return lbool.Undef, e
	}
	// Failed scoped_append construction has no source destructor. Completed
	// append restores size only after its actual nested Check returns/unwinds.
	if e = satMUSAppend(&m.mus, core); e != nil {
		return lbool.Undef, e
	}
	defer func() {
		if cleanup := satMUSResize(&m.mus, n); cleanup != nil {
			e = errors.Join(e, cleanup)
		}
	}()
	if e = m.mus.Push(uint32(lit.Negated())); e != nil {
		return lbool.Undef, e
	}
	count, e := m.mus.Size()
	if e != nil {
		return lbool.Undef, e
	}
	r, e = m.solver.checkSpan(count, satConflictColumnSpan(&m.mus))
	if e == nil {
		m.traceMUSCore(93, "mus: ", &m.mus)
	}
	return r, e
}
func (m *satSolverMUSMembers) mus1() (lbool.LBool, error) {
	partial := m.solver.config.state.CoreMinimizePartial
	m.traceMUSCore(61, "old core: ", &m.solver.conflict.core)
	if e := m.getCore(); e != nil {
		return lbool.Undef, e
	}
	n, e := m.core.Size()
	if e != nil {
		return lbool.Undef, e
	}
	if !partial && n > 64 {
		return m.mus2()
	}
	for {
		n, e = m.core.Size()
		if e != nil {
			return lbool.Undef, e
		}
		if n == 0 {
			break
		}
		if e = m.emitMUSProgress(n, partial); e != nil {
			return lbool.Undef, e
		}
		m.traceMUSPair(71, "core: ", &m.core, "mus:  ", &m.mus)
		if !m.solver.Limit.Inc() {
			if e = m.setCore(); e != nil {
				return lbool.Undef, e
			}
			return lbool.Undef, nil
		}
		k, e := m.mus.Size()
		if e != nil {
			return lbool.Undef, e
		}
		if n+k <= 2 {
			break
		}
		v, e := m.core.Read(n - 1)
		if e != nil {
			return lbool.Undef, e
		}
		if e = m.core.Shrink(n - 1); e != nil {
			return lbool.Undef, e
		}
		lit := Literal(v)
		result, e := m.scopedCheck(&m.core, lit)
		if e != nil {
			return lbool.Undef, e
		}
		if e = m.emitMUSResult(result); e != nil {
			return lbool.Undef, e
		}
		switch result {
		case lbool.Undef:
			if m.solver.Limit.Inc() {
				if e = m.mus.Push(v); e != nil {
					return lbool.Undef, e
				}
			} else {
				if e = m.core.Push(v); e != nil {
					return lbool.Undef, e
				}
				if e = m.setCore(); e != nil {
					return lbool.Undef, e
				}
				return lbool.Undef, nil
			}
		case lbool.True:
			if satStorageSourceDebug {
				a, e := m.solver.model.Read(uint32(lit.variable()))
				if e != nil {
					return lbool.Undef, e
				}
				value := lbool.LBool(int32(a))
				if lit.sign() {
					value = value.Negated()
				}
				if value != lbool.False {
					return lbool.Undef, ErrSATAssertion
				}
			}
			if e = m.mus.Push(v); e != nil {
				return lbool.Undef, e
			}
			if e = m.updateModel(); e != nil {
				return lbool.Undef, e
			}
		case lbool.False:
			yes, e := satMUSContains(&m.solver.conflict.core, uint32(lit.Negated()))
			if e != nil {
				return lbool.Undef, e
			}
			if yes {
				if e = m.emitMUSUnitReduction(lit); e != nil {
					return lbool.Undef, e
				}
			} else {
				m.traceMUSPair(120, "core: ", &m.solver.conflict.core, " mus: ", &m.mus)
				if e = m.core.Shrink(0); e != nil {
					return lbool.Undef, e
				}
				for i := uint32(0); ; i++ {
					sz, e := m.solver.conflict.core.Size()
					if e != nil {
						return lbool.Undef, e
					}
					if i >= sz {
						break
					}
					v, e := m.solver.conflict.core.Read(i)
					if e != nil {
						return lbool.Undef, e
					}
					yes, e := satMUSContains(&m.mus, v)
					if e != nil {
						return lbool.Undef, e
					}
					if !yes {
						if e = m.core.Push(v); e != nil {
							return lbool.Undef, e
						}
					}
				}
			}
		}
	}
	if e = m.setCore(); e != nil {
		return lbool.Undef, e
	}
	if e = m.emitMUSNewCore(); e != nil {
		return lbool.Undef, e
	}
	return lbool.True, nil
}
func (m *satSolverMUSMembers) setFromColumn(dst *satMUSLiteralSet, src *satUintColumn) error {
	for i := uint32(0); ; i++ {
		n, e := src.Size()
		if e != nil {
			return e
		}
		if i >= n {
			return nil
		}
		v, e := src.Read(i)
		if e != nil {
			return e
		}
		if e = dst.insert(Literal(v)); e != nil {
			return e
		}
	}
}
func (m *satSolverMUSMembers) toVector(set *satMUSLiteralSet, dst *satUintColumn) error {
	// Named return-value move is explicit: one source result vector, whose
	// lexical caller owns its destruction. Captured begin/end never rebind.
	n := set.size()
	span := satSolverLiteralSpan{literals: set.values}
	for i := uint32(0); i < n; i++ {
		l, e := span.literal(i)
		if e != nil {
			return e
		}
		if e = dst.Push(uint32(l)); e != nil {
			return e
		}
	}
	return nil
}
func (m *satSolverMUSMembers) mus2() (r lbool.LBool, e error) {
	f, e := m.frame(2, true)
	if e != nil {
		return lbool.Undef, e
	}
	defer func() { e = m.drop(f, e) }()
	if e = m.getCore(); e != nil {
		return lbool.Undef, e
	}
	if e = m.setFromColumn(&f.sets[0], &m.core); e != nil {
		return lbool.Undef, e
	}
	r, e = m.qx(&f.sets[0], &f.sets[1], false)
	if e != nil {
		return lbool.Undef, e
	}
	if e = m.solver.conflict.core.Shrink(0); e != nil {
		return lbool.Undef, e
	}
	if e = m.toVector(&f.sets[0], &f.vector); e != nil {
		return lbool.Undef, e
	}
	if e = satMUSAppend(&m.solver.conflict.core, &f.vector); e != nil {
		return r, e
	}
	return r, m.emitMUSNewCore()
}
func (m *satSolverMUSMembers) split(a, b *satMUSLiteralSet) (e error) {
	n := a.size()
	half := n / 2
	f, e := m.frame(1, false)
	if e != nil {
		return e
	}
	defer func() { e = m.drop(f, e) }()
	span := satSolverLiteralSpan{literals: a.values}
	for i := uint32(0); i < n; i++ {
		v, e := span.literal(i)
		if e != nil {
			return e
		}
		dst := b
		if i < half {
			dst = &f.sets[0]
		}
		if e = dst.insert(v); e != nil {
			return e
		}
	}
	a.assign(&f.sets[0])
	return nil
}
func (m *satSolverMUSMembers) unsplit(a, b *satMUSLiteralSet) (e error) {
	f, e := m.frame(2, false)
	if e != nil {
		return e
	}
	defer func() { e = m.drop(f, e) }()
	n := a.size()
	span := satSolverLiteralSpan{literals: a.values}
	for i := uint32(0); i < n; i++ {
		v, e := span.literal(i)
		if e != nil {
			return e
		}
		yes := b.contains(v)
		idx := 0
		if yes {
			idx = 1
		}
		if e = f.sets[idx].insert(v); e != nil {
			return e
		}
	}
	a.assign(&f.sets[0])
	b.assign(&f.sets[1])
	return nil
}
func (m *satSolverMUSMembers) qx(a, b *satMUSLiteralSet, support bool) (r lbool.LBool, e error) {
	r = lbool.True
	if support {
		r, e = m.qxCheck(a, b)
		if e != nil {
			return r, e
		}
		if r == lbool.False {
			return lbool.True, nil
		}
		if r != lbool.True {
			return r, nil
		}
	}
	n := a.size()
	if n == 1 {
		return lbool.True, nil
	}
	f, e := m.frame(1, false)
	if e != nil {
		return lbool.Undef, e
	}
	defer func() { e = m.drop(f, e) }()
	a2 := &f.sets[0]
	if e = m.split(a, a2); e != nil {
		return lbool.Undef, e
	}
	if e = b.union(a); e != nil {
		return lbool.Undef, e
	}
	n = a.size()
	if n == 0 {
		second := a2.size()
		// The first split and union have completed. The next identical empty
		// recursive call has no finite source continuation; preserve that prefix.
		if second == 0 {
			return lbool.Undef, ErrSATMUSEmptyRecursion
		}
	}
	r, e = m.qx(a2, b, n != 0)
	if e != nil {
		return r, e
	}
	if e = m.unsplit(b, a); e != nil {
		return lbool.Undef, e
	}
	if r != lbool.True {
		return r, nil
	}
	if e = b.union(a2); e != nil {
		return lbool.Undef, e
	}
	n = a2.size()
	r, e = m.qx(a, b, n != 0)
	if e != nil {
		return r, e
	}
	if e = a.union(a2); e != nil {
		return lbool.Undef, e
	}
	if e = m.unsplit(b, a2); e != nil {
		return lbool.Undef, e
	}
	return r, nil
}
func (m *satSolverMUSMembers) qxCheck(a, b *satMUSLiteralSet) (r lbool.LBool, e error) {
	f, e := m.frame(0, true)
	if e != nil {
		return lbool.Undef, e
	}
	defer func() { e = m.drop(f, e) }()
	if e = m.toVector(b, &f.vector); e != nil {
		return lbool.Undef, e
	}
	old, e := m.mus.Size()
	if e != nil {
		return lbool.Undef, e
	}
	if e = satMUSAppend(&m.mus, &f.vector); e != nil {
		return lbool.Undef, e
	}
	f.appendActive = true
	f.appendOld = old
	// Full-expression temporary is destroyed BEFORE the following Check body.
	if e = f.vector.Finalize(); e != nil {
		f.cleanupStopped = true
		return lbool.Undef, e
	}
	f.vectorUsed = false
	n, e := m.mus.Size()
	if e != nil {
		return lbool.Undef, e
	}
	r, e = m.solver.checkSpan(n, satConflictColumnSpan(&m.mus))
	if e != nil {
		return r, e
	}
	switch r {
	case lbool.False:
		f.setsUsed = 1
		if e = m.setFromColumn(&f.sets[0], &m.solver.conflict.core); e != nil {
			return lbool.Undef, e
		}
		f.innerCoreCompleted = true
		b.intersect(&f.sets[0])
		a.reset()
		return lbool.False, nil
	case lbool.True:
		if e = m.updateModel(); e != nil {
			return lbool.Undef, e
		}
	}
	return r, nil
}

func (s *Solver) setModel(src *satUintColumn) error {
	if e := s.model.Shrink(0); e != nil {
		return e
	}
	if e := satMUSAppend(&s.model, src); e != nil {
		return e
	}
	n, e := s.model.Size()
	if e != nil {
		return e
	}
	s.Variables.modelIsCurrent = n != 0
	return nil
}

func (m *satSolverMUSMembers) verifyCore(core *satUintColumn) error {
	n, e := core.Size()
	if e != nil {
		return e
	}
	_, e = m.solver.checkSpan(n, satConflictColumnSpan(core))
	return e
}
func (m *satSolverMUSMembers) isActiveSource() bool     { return m.isActive }
func (m *satSolverMUSMembers) getModel() *satUintColumn { return &m.model }

// Scoped append restores its saved length, including value-initialized growth
// if the borrower shrank the native slice during the call.
func satMUSResize(v *satUintColumn, n uint32) error { return v.resize(n, 0) }
