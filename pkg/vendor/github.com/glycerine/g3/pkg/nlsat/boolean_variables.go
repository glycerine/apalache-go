package nlsat

// Source: src/nlsat/nlsat_solver.cpp:90-97,395-409,467-474.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/idgen"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// booleanVariables holds the actual solver parallel arrays. Native slices
// replace source vector::setx, without emulating its allocation machinery.
type booleanVariables struct {
	ids            idgen.Generator
	count          uint32
	atoms          []ArithmeticAtom
	values         []lbool.LBool
	levels         []uint32
	justifications []Justification
	watches        [][]*Clause
	dead           []bool
	recycling      sat.BoolVar
}

func (v *booleanVariables) initialize(runtime *umemory.Runtime) {
	v.ids = idgen.New(0, func() bool { return runtime.State().OutOfMemory })
	v.recycling = sat.NullBoolVar
}

func (v *booleanVariables) mk() (sat.BoolVar, error) {
	if v.recycling != sat.NullBoolVar {
		return sat.NullBoolVar, ErrAtomCachePending
	}
	b := sat.BoolVar(v.ids.Mk())
	v.count++
	for uint64(len(v.atoms)) <= uint64(b) {
		v.atoms = append(v.atoms, nil)
	}
	v.atoms[b] = nil
	for uint64(len(v.values)) <= uint64(b) {
		v.values = append(v.values, lbool.Undef)
	}
	v.values[b] = lbool.Undef
	for uint64(len(v.levels)) <= uint64(b) {
		v.levels = append(v.levels, ^uint32(0))
	}
	v.levels[b] = ^uint32(0)
	for uint64(len(v.justifications)) <= uint64(b) {
		v.justifications = append(v.justifications, Justification{})
	}
	v.justifications[b] = Justification{}
	for uint64(len(v.watches)) <= uint64(b) {
		v.watches = append(v.watches, nil)
	}
	v.watches[b] = nil
	for uint64(len(v.dead)) <= uint64(b) {
		v.dead = append(v.dead, true)
	}
	v.dead[b] = false
	return b, nil
}

func (v *booleanVariables) erase(b sat.BoolVar) error {
	if v.recycling == sat.NullBoolVar {
		if uint64(b) >= uint64(len(v.dead)) || v.dead[b] {
			return arena.ErrBounds
		}
		if err := atomAssert("nlsat_solver.cpp", 468, "m_bwatches[b].empty()", len(v.watches[b]) == 0); err != nil {
			return err
		}
		if len(v.watches[b]) != 0 {
			return ErrAtomCacheLive
		}
		v.count--
		v.dead[b] = true
		v.atoms[b] = nil
		v.recycling = b
	} else if v.recycling != b {
		return ErrAtomCachePending
	}
	if err := v.ids.Recycle(uint32(b)); err != nil {
		return err
	}
	v.recycling = sat.NullBoolVar
	return nil
}

func arithmeticAtomHeader(a ArithmeticAtom) *Atom {
	switch a := a.(type) {
	case *InequalityAtom:
		if a != nil {
			return &a.Atom
		}
	case *RootAtom:
		if a != nil {
			return &a.Atom
		}
	}
	return nil
}
