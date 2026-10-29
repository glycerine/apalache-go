package sat

// Source: src/sat/sat_solver.cpp:3252–3636; src/util/vector.h; src/util/hashtable.h.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// A source vector borrow: a real member uses current size/operator[] each time,
// while c_ptr() captures its actual payload only at the source call site. The
// caller-array forms are explicit concrete input-boundary adapters, not owners.
type satConsequenceInput struct {
	column    *satUintColumn
	literals  []Literal
	variables []BoolVar
	raw       satSolverLiteralSpan
	count     uint32
}

func (v satConsequenceInput) size() (uint32, error) {
	if v.column != nil {
		return v.column.Size()
	}
	return v.count, nil
}
func (v satConsequenceInput) read(i uint32) (uint32, error) {
	if v.column != nil {
		return v.column.Read(i)
	}
	if v.variables != nil {
		if uint64(i) >= uint64(cap(v.variables)) {
			return 0, arena.ErrBounds
		}
		return uint32(v.variables[:cap(v.variables)][i]), nil
	}
	span := v.raw
	if v.literals != nil {
		span = satSolverLiteralSpan{literals: v.literals}
	}
	l, e := span.literal(i)
	return l.index(), e
}
func (v satConsequenceInput) cPtr() satSolverLiteralSpan {
	if v.column != nil {
		return satConflictColumnSpan(v.column)
	}
	if v.literals != nil {
		return satSolverLiteralSpan{literals: v.literals}
	}
	return v.raw
}
func satConsequenceLiterals(v []Literal) satConsequenceInput {
	return satConsequenceInput{literals: v, count: uint32(len(v))}
}
func satConsequenceColumn(v *satUintColumn) satConsequenceInput {
	return satConsequenceInput{column: v}
}

type satConsequenceFrame struct {
	next                *satConsequenceFrame
	sets                [3]satMUSLiteralSet
	vectors             [3]satUintColumn
	index               satSolverIndexSet
	order               [7]uint8
	completed           [7]bool
	used, phase, cursor uint32
	bytes               uint64
}

func (s *Solver) consequenceFrame() (*satConsequenceFrame, error) {
	bytes := uint64(reflect.TypeOf(satConsequenceFrame{}).Size())
	if e := s.budget.Reserve(bytes); e != nil {
		return nil, e
	}
	f := &satConsequenceFrame{next: s.consequenceFrames, bytes: bytes}
	s.consequenceFrames = f
	return f, nil
}
func (s *Solver) consequenceDrop(f *satConsequenceFrame, cause error) error {
	if s.consequenceFrames != f {
		if cause != nil {
			return cause
		}
		return ErrSATOwner
	}
	// First actual completed source local destructors in reverse declaration
	// order. Only afterwards drain failed constructor buffers as owning abort.
	for f.phase < 2 {
		for f.cursor < f.used {
			i := f.used - 1 - f.cursor
			if f.completed[i] != (f.phase == 0) {
				f.cursor++
				continue
			}
			tag := f.order[i]
			var e error
			if tag < 3 {
				f.sets[tag].close()
			} else if tag < 6 {
				e = f.vectors[tag-3].Finalize()
			} else {
				e = f.index.close()
			}
			if e != nil {
				if cause != nil {
					return cause
				}
				return e
			}
			f.cursor++
		}
		f.phase++
		f.cursor = 0
	}
	s.consequenceFrames = f.next
	s.budget.Release(f.bytes)
	return cause
}
func (s *Solver) cleanupConsequences() error {
	for s.consequenceFrames != nil {
		if e := s.consequenceDrop(s.consequenceFrames, nil); e != nil {
			return e
		}
	}
	return nil
}
func (f *satConsequenceFrame) set(s *Solver, i uint8, input *satConsequenceInput) error {
	f.order[f.used] = i
	f.completed[f.used] = true
	f.used++
	set := &f.sets[i]
	if input == nil {
		return nil
	}
	for j := uint32(0); ; j++ {
		n, e := input.size()
		if e != nil {
			return e
		}
		if j >= n {
			return nil
		}
		v, e := input.read(j)
		if e != nil {
			return e
		}
		if e = set.insert(Literal(v)); e != nil {
			return e
		}
	}
}
func (f *satConsequenceFrame) vector(s *Solver, i uint8, copy *satUintColumn) error {
	k := f.used
	f.order[k] = i + 3
	f.completed[k] = copy == nil
	f.used++
	f.vectors[i].Bind(s.runtime.memory)
	if copy != nil {
		if e := f.vectors[i].copyFrom(copy); e != nil {
			return e
		}
		f.completed[k] = true
	}
	return nil
}
func (f *satConsequenceFrame) indexSet() {
	k := f.used
	f.order[k] = 6
	f.used++
	f.index = satSolverIndexSet{}
	f.completed[k] = true
}
func satConsequenceSetRemove(s *satMUSLiteralSet, v uint32) error {
	if !s.contains(Literal(v)) {
		return nil
	}
	s.membership[v] = false
	for i, l := range s.values {
		if l.index() == v {
			s.values[i] = s.values[len(s.values)-1]
			s.values = s.values[:len(s.values)-1]
			return nil
		}
	}
	return ErrSATAssertion
}
