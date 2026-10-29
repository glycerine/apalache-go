package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import (
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	uhash "github.com/glycerine/g3/pkg/util/hash"
)

func (m *MonomialManager) writePower(a Monomial, i uint32, p MonomialPower) error {
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if e = m.allocator.WriteUint32(l.raw, 20+uint64(i)*8, p.Var); e != nil {
		return e
	}
	return m.allocator.WriteUint32(l.raw, 24+uint64(i)*8, p.Degree)
}
func (m *MonomialManager) renameObject(a Monomial, xs []uint32) error {
	n, e := m.sizeCore(a)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		v, e := m.powerFieldCore(a, i, 0)
		if e != nil {
			return e
		}
		if uint64(v) >= uint64(len(xs)) {
			return arena.ErrBounds
		}
		l, e := m.location(a)
		if e != nil {
			return e
		}
		if e = m.allocator.WriteUint32(l.raw, 20+uint64(i)*8, xs[v]); e != nil {
			return e
		}
	}
	if n > 1 {
		powers := make([]MonomialPower, n)
		for i := range powers {
			powers[i].Var, e = m.powerFieldCore(a, uint32(i), 0)
			if e != nil {
				return e
			}
			powers[i].Degree, e = m.powerFieldCore(a, uint32(i), 4)
			if e != nil {
				return e
			}
		}
		sort.Slice(powers, func(i, j int) bool { return powers[i].Var < powers[j].Var })
		for i, p := range powers {
			if e = m.writePower(a, uint32(i), p); e != nil {
				return e
			}
		}
	}
	var problem error
	hash := uhash.WordReader(n*8, 11, func(i uint32) uint32 {
		l, e := m.location(a)
		if e != nil {
			problem = e
			return 0
		}
		v, e := m.allocator.ReadUint32(l.raw, 20+uint64(i)*4)
		if e != nil {
			problem = e
		}
		return v
	})
	if problem != nil {
		return problem
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	return m.allocator.WriteUint32(l.raw, 16, hash)
}
func (m *MonomialManager) Rename(xs []uint32) (err error) {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceScalarAssert(1271, "m_ref_count <= 1", m.managerRefs <= 1); e != nil {
		return e
	}
	if e := m.sourceScalarAssert(1272, "sz == num_vars()", uint64(len(xs)) == uint64(m.nextVar)); e != nil {
		return e
	}
	if m.renameTable.entries.Len() != 0 || m.renameFound != nil {
		return ErrMonomialPending
	}
	if polynomialSourceDebug {
		m.renameFound = make([]bool, m.nextVar)
		for _, x := range xs {
			if e := m.sourceScalarAssert(1278, "xs[i] < num_vars()", x < m.nextVar); e != nil {
				return e
			}
			if x >= m.nextVar {
				return arena.ErrBounds
			}
			if e := m.sourceScalarAssert(1279, "!found[xs[i]]", !m.renameFound[x]); e != nil {
				return e
			}
			m.renameFound[x] = true
		}
		m.renameFound = nil
	}
	t := &m.renameTable
	defer t.cleanup()
	for _, entry := range m.table.entries.All() {
		k := entry.key
		if k.kind != monomialObjectKey {
			return ErrMonomialPending
		}
		a := Monomial{m.origin, k.slot, m.locations[k.slot-1].generation}
		if e := m.renameObject(a, xs); e != nil {
			return e
		}
		if e := m.renameUniqueAssert(t, k); e != nil {
			return e
		}
		if e := t.insertRenamed(m, k); e != nil {
			return e
		}
	}
	m.table, m.renameTable = m.renameTable, m.table
	return nil
}
func (m *MonomialManager) CleanupRename() error {
	if m.substitutionTraceCallback() || (m.exactDivTraceCallback() || m.lexTraceCallback()) {
		return ErrMonomialPending
	}
	if m == nil || m.self != m || m.closed {
		return ErrMonomialOwner
	}
	m.renameFound = nil
	m.renameTable.cleanup()
	return nil
}
