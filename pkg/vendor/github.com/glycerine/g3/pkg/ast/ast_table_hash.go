package ast

// Exact source AST array hash and compare_nodes/get_node_hash branches.

import (
	"github.com/glycerine/g3/pkg/arena"
	uhash "github.com/glycerine/g3/pkg/util/hash"
)

func astHashArray(n, init uint32, hash func(uint32) uint32) uint32 {
	switch n {
	case 0:
		return init
	case 1:
		return uhash.Combine(hash(0), init)
	case 2:
		return uhash.Combine(uhash.Combine(hash(0), hash(1)), init)
	case 3:
		return uhash.Combine(uhash.Combine(hash(0), hash(1)), uhash.Combine(hash(2), init))
	}
	a, b, c := uint32(0x9e3779b9), uint32(0x9e3779b9), init
	for n >= 3 {
		n--
		a += hash(n)
		n--
		b += hash(n)
		n--
		c += hash(n)
		a, b, c = uhash.MixState(a, b, c)
	}
	if n == 2 {
		b += hash(1)
	}
	if n >= 1 {
		c += hash(0)
	}
	return uhash.Mix(a, b, c)
}
func (m *ASTManager) hashPayload(p arena.SmallBytesRef, count, init uint32) (uint32, error) {
	var err error
	result := astHashArray(count, init, func(i uint32) uint32 {
		r, e := m.readInternal(p, i)
		if e == nil {
			e = m.validNode(r)
		}
		if e != nil {
			if err == nil {
				err = e
			}
			return 0
		}
		return m.nodeAt(r.slot).hash
	})
	return result, err
}
func (m *ASTManager) nodeHash(r astNodeRef) (uint32, error) {
	if e := m.validNode(r); e != nil {
		return 0, e
	}
	switch m.nodeAt(r.slot).kind {
	case ASTSort:
		v := m.sortAt(r.slot)
		h, e := m.Symbols.Hash(v.name)
		if e != nil {
			return 0, e
		}
		if v.info == (ASTSortInfo{}) {
			return h, nil
		}
		ih, e := m.infoHash(v.info.slot, v.info.generation)
		return uhash.Combine(h, ih), e
	case ASTFuncDecl:
		v := m.declAt(r.slot)
		h, e := m.Symbols.Hash(v.name)
		if e != nil {
			return 0, e
		}
		if v.info != (ASTFuncDeclInfo{}) {
			ih, e := m.infoHash(v.info.slot, v.info.generation)
			if e != nil {
				return 0, e
			}
			h = uhash.Combine(h, ih)
		}
		return m.hashPayload(v.domain, v.arity, h)
	case ASTApp:
		v := m.appAt(r.slot)
		if e := m.validNode(v.decl); e != nil {
			return 0, e
		}
		return m.hashPayload(v.args, v.count, m.nodeAt(v.decl.slot).hash)
	case ASTVar:
		v := m.varAt(r.slot)
		if e := m.validNode(v.sort); e != nil {
			return 0, e
		}
		return uhash.Combine(v.index, m.nodeAt(v.sort.slot).hash), nil
	case ASTQuantifier:
		v := m.quantifierAt(r.slot)
		init := uint32(19)
		if v.forall {
			init = 31
		}
		a, e := m.hashPayload(v.sorts, v.count, init)
		if e != nil {
			return 0, e
		}
		if e = m.validNode(v.body); e != nil {
			return 0, e
		}
		return uhash.Mix(a, v.patternCount, m.nodeAt(v.body.slot).hash), nil
	}
	return 0, ErrASTDomain
}
func (m *ASTManager) equalPayload(a, b arena.SmallBytesRef, n uint32) (bool, error) {
	for i := uint32(0); i < n; i++ {
		x, e := m.readInternal(a, i)
		if e != nil {
			return false, e
		}
		y, e := m.readInternal(b, i)
		if e != nil {
			return false, e
		}
		if x != y {
			return false, nil
		}
	}
	return true, nil
}
func (m *ASTManager) compareNodes(a, b astNodeRef) (bool, error) {
	if e := m.validNode(a); e != nil {
		return false, e
	}
	if e := m.validNode(b); e != nil {
		return false, e
	}
	kind := m.nodeAt(a.slot).kind
	if kind != m.nodeAt(b.slot).kind {
		return false, nil
	}
	switch kind {
	case ASTSort:
		x, y := m.sortAt(a.slot), m.sortAt(b.slot)
		if (x.info == ASTSortInfo{}) != (y.info == ASTSortInfo{}) {
			return false, nil
		}
		if x.info != (ASTSortInfo{}) {
			eq, e := m.infoEqual(x.info.slot, x.info.generation, y.info.slot, y.info.generation)
			if e != nil || !eq {
				return eq, e
			}
		}
		return x.name == y.name, nil
	case ASTFuncDecl:
		x, y := m.declAt(a.slot), m.declAt(b.slot)
		if (x.info == ASTFuncDeclInfo{}) != (y.info == ASTFuncDeclInfo{}) {
			return false, nil
		}
		if x.info != (ASTFuncDeclInfo{}) {
			eq, e := m.infoEqual(x.info.slot, x.info.generation, y.info.slot, y.info.generation)
			if e != nil || !eq {
				return eq, e
			}
		}
		if x.name != y.name || x.arity != y.arity || x.rangeSort != y.rangeSort {
			return false, nil
		}
		return m.equalPayload(x.domain, y.domain, x.arity)
	case ASTApp:
		x, y := m.appAt(a.slot), m.appAt(b.slot)
		if x.decl != y.decl || x.count != y.count {
			return false, nil
		}
		return m.equalPayload(x.args, y.args, x.count)
	case ASTVar:
		x, y := m.varAt(a.slot), m.varAt(b.slot)
		return x.index == y.index && x.sort == y.sort, nil
	case ASTQuantifier:
		x, y := m.quantifierAt(a.slot), m.quantifierAt(b.slot)
		if x.forall != y.forall || x.count != y.count {
			return false, nil
		}
		same, e := m.equalPayload(x.sorts, y.sorts, x.count)
		if e != nil || !same {
			return same, e
		}
		for i := uint32(0); i < x.count; i++ {
			s, e := m.readSymbol(x.names, i)
			if e != nil {
				return false, e
			}
			t, e := m.readSymbol(y.names, i)
			if e != nil {
				return false, e
			}
			if s != t {
				return false, nil
			}
		}
		if x.body != y.body || x.weight != y.weight || x.patternCount != y.patternCount {
			return false, nil
		}
		same, e = m.equalPayload(x.patterns, y.patterns, x.patternCount)
		if e != nil || !same {
			return same, e
		}
		if x.noPatternCount != y.noPatternCount {
			return false, nil
		}
		return m.equalPayload(x.noPatterns, y.noPatterns, x.noPatternCount)
	}
	return false, ErrASTDomain
}
