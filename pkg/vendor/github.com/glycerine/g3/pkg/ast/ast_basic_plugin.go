package ast

// Actual basic_decl_plugin constructor, lazy declaration caches, and dispatch.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (m *ASTManager) basicSort(name string, k int32, size SortSize) (astNodeRef, error) {
	sym, e := m.Symbols.Intern(name)
	if e != nil {
		return astNodeRef{}, e
	}
	s, g, e := m.newInfo(true, 0, k, nil, size, ASTFuncFlags{}, false)
	if e != nil {
		return astNodeRef{}, e
	}
	defer m.dropInfoStorage(s, g)
	r, e := m.mkSort(sym, ASTSortInfo{m.origin, s, g})
	if e == nil {
		e = m.incNode(r)
	}
	return r, e
}
func (m *ASTManager) basicNamedDecl(name string, k int32, domain []ASTHandle, result astNodeRef, flags ASTFuncFlags, ps []ASTParameter, pin bool) (astNodeRef, error) {
	sym, e := m.Symbols.Intern(name)
	if e != nil {
		return astNodeRef{}, e
	}
	r, e := m.pluginDecl(sym, 0, k, ps, domain, result, flags, false)
	if e == nil && pin {
		e = m.incNode(r)
	}
	return r, e
}
func (m *ASTManager) initBasic() error {
	r, e := m.basicSort("Bool", 0, FiniteSortSize(2))
	if e != nil {
		return e
	}
	m.basic.boolSort = r
	var domain [2]ASTHandle
	domain[0] = m.external(r)
	domain[1] = m.external(r)
	// Lexical construction order differs from finalize order in the source.
	for _, op := range []struct {
		name  string
		k     int32
		n     int
		flags ASTFuncFlags
	}{
		{"true", ASTOpTrue, 0, ASTFuncFlags{}}, {"false", ASTOpFalse, 0, ASTFuncFlags{}},
		{"and", ASTOpAnd, 2, ASTFuncFlags{LeftAssociative: true, RightAssociative: true, FlatAssociative: true, Commutative: true, Idempotent: true}},
		{"or", ASTOpOr, 2, ASTFuncFlags{LeftAssociative: true, RightAssociative: true, FlatAssociative: true, Commutative: true, Idempotent: true}},
		{"iff", ASTOpIff, 2, ASTFuncFlags{Commutative: true, Chainable: true}},
		{"xor", ASTOpXor, 2, ASTFuncFlags{LeftAssociative: true, RightAssociative: true, Commutative: true}},
		{"not", ASTOpNot, 1, ASTFuncFlags{}}, {"interp", ASTOpInterp, 1, ASTFuncFlags{}},
		{"=>", ASTOpImplies, 2, ASTFuncFlags{RightAssociative: true}},
	} {
		r, e = m.basicNamedDecl(op.name, op.k, domain[:op.n], m.basic.boolSort, op.flags, nil, true)
		if e != nil {
			return e
		}
		m.basic.declarations[op.k] = r
	}
	r, e = m.basicSort("Proof", 1, InfiniteSortSize())
	if e != nil {
		return e
	}
	m.basic.proofSort = r
	r, e = m.basicNamedDecl("undef", ASTPrUndef, nil, r, ASTFuncFlags{}, nil, true)
	if e != nil {
		return e
	}
	m.basic.declarations[ASTPrUndef] = r
	return nil
}
func (m *ASTManager) resizeDeclCache(cache *[]astNodeRef, needed uint32) error {
	if uint64(needed) <= uint64(len(*cache)) {
		return nil
	}
	if uint64(needed) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	*cache = append(*cache, make([]astNodeRef, int(needed)-len(*cache))...)
	return nil
}
func (m *ASTManager) joinSorts(a, b astNodeRef) (astNodeRef, error) {
	if a == b {
		return a, nil
	}
	af, ak, e := m.declIdentity(a)
	if e != nil {
		return astNodeRef{}, e
	}
	bf, _, e := m.declIdentity(b)
	if e != nil {
		return astNodeRef{}, e
	}
	if m.arithmeticPlugin != nil && af == m.arithmeticPlugin.family && bf == af {
		if ak == 0 {
			return a, nil
		}
		return b, nil
	}
	return astNodeRef{}, ErrASTDomain
}
func (m *ASTManager) cachedEquality(k int32, sort astNodeRef) (astNodeRef, error) {
	id := m.nodeAt(sort.slot).id - (1 << 31)
	if id == math.MaxUint32 {
		return astNodeRef{}, arena.ErrBounds
	}
	cache := &m.basic.eq
	name := "="
	if k == ASTOpOEq {
		cache = &m.basic.oeq
		name = "~"
	}
	if e := m.resizeDeclCache(cache, id+1); e != nil {
		return astNodeRef{}, e
	}
	if (*cache)[id].slot == 0 {
		domain := [2]ASTHandle{m.external(sort), m.external(sort)}
		r, e := m.basicNamedDecl(name, k, domain[:], m.basic.boolSort, ASTFuncFlags{Commutative: true, Chainable: true}, nil, false)
		if e != nil {
			return astNodeRef{}, e
		}
		(*cache)[id] = r
		if e = m.incNode(r); e != nil {
			return astNodeRef{}, e
		}
	}
	return (*cache)[id], nil
}
func (m *ASTManager) cachedIte(sort astNodeRef) (astNodeRef, error) {
	id := m.nodeAt(sort.slot).id - (1 << 31)
	if id == math.MaxUint32 {
		return astNodeRef{}, arena.ErrBounds
	}
	if e := m.resizeDeclCache(&m.basic.ite, id+1); e != nil {
		return astNodeRef{}, e
	}
	if m.basic.ite[id].slot == 0 {
		domain := [3]ASTHandle{m.external(m.basic.boolSort), m.external(sort), m.external(sort)}
		r, e := m.basicNamedDecl("if", ASTOpIte, domain[:], sort, ASTFuncFlags{}, nil, false)
		if e != nil {
			return astNodeRef{}, e
		}
		m.basic.ite[id] = r
		if e = m.incNode(r); e != nil {
			return astNodeRef{}, e
		}
	}
	return m.basic.ite[id], nil
}
func (m *ASTManager) basicDecl(k int32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	switch k {
	case ASTOpTrue, ASTOpFalse, ASTOpAnd, ASTOpOr, ASTOpIff, ASTOpXor, ASTOpNot, ASTOpInterp, ASTOpImplies:
		return m.basic.declarations[k], nil
	case ASTOpIte:
		if len(domain) != 3 {
			return astNodeRef{}, nil
		}
		a, e := m.requireKind(domain[1], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		b, e := m.requireKind(domain[2], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		r, e := m.joinSorts(a, b)
		if e != nil {
			return astNodeRef{}, e
		}
		return m.cachedIte(r)
	case ASTOpEq, ASTOpOEq:
		if len(domain) < 2 {
			return astNodeRef{}, nil
		}
		r, e := m.requireKind(domain[0], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		for _, h := range domain[1:] {
			other, e := m.requireKind(h, ASTSort)
			if e != nil {
				return astNodeRef{}, e
			}
			r, e = m.joinSorts(r, other)
			if e != nil {
				return astNodeRef{}, e
			}
		}
		return m.cachedEquality(k, r)
	case ASTOpDistinct:
		for i := 1; i < len(domain); i++ {
			if domain[i] != domain[0] {
				return astNodeRef{}, ErrASTDomain
			}
		}
		return m.basicNamedDecl("distinct", k, domain, m.basic.boolSort, ASTFuncFlags{Pairwise: true}, nil, false)
	}
	if k < ASTPrUndef || k >= ASTLastBasicPR {
		return astNodeRef{}, ErrASTDomain
	}
	if k == ASTPrUndef {
		if len(domain) != 0 {
			return astNodeRef{}, ErrASTDomain
		}
	} else {
		if len(domain) == 0 {
			return astNodeRef{}, ErrASTDomain
		}
		for _, h := range domain[:len(domain)-1] {
			r, e := m.requireKind(h, ASTSort)
			if e != nil || r != m.basic.proofSort {
				return astNodeRef{}, ErrASTDomain
			}
		}
		last, e := m.requireKind(domain[len(domain)-1], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		if last != m.basic.boolSort && last != m.basic.proofSort {
			return astNodeRef{}, ErrASTDomain
		}
	}
	return m.proofDecl(k, ps, uint32(len(domain))-1)
}
