package sat

// Source: src/sat/sat_big.cpp:160-226.

import "github.com/glycerine/g3/pkg/arena"

func (b *satBIG) reaches(u, v Literal) bool {
	if !b.validLiteral(u) || !b.validLiteral(v) || len(b.left) != len(b.dag) {
		return false
	}
	return b.left[u.index()] < b.left[v.index()] && b.right[v.index()] < b.right[u.index()]
}

func (b *satBIG) connected(u, v Literal) bool {
	return b.reaches(u, v) || b.reaches(v.Negated(), u.Negated())
}

func (b *satBIG) getLeft(l Literal) int32 {
	if !b.validLiteral(l) || len(b.left) != len(b.dag) {
		return 0
	}
	return b.left[l.index()]
}

func (b *satBIG) getRight(l Literal) int32 {
	if !b.validLiteral(l) || len(b.right) != len(b.dag) {
		return 0
	}
	return b.right[l.index()]
}

func (b *satBIG) getParent(l Literal) Literal {
	if !b.validLiteral(l) || len(b.parent) != len(b.dag) {
		return DefaultLiteral()
	}
	return b.parent[l.index()]
}

func (b *satBIG) getRoot(l Literal) (Literal, error) {
	if !b.validLiteral(l) || len(b.root) != len(b.dag) {
		return DefaultLiteral(), arena.ErrBounds
	}
	r := l
	for {
		l = r
		r = b.root[l.index()]
		if r == l {
			return r, nil
		}
	}
}

func (b *satBIG) next(u, v Literal) (Literal, error) {
	if !b.reaches(u, v) {
		return DefaultLiteral(), ErrSATAssertion
	}
	result := DefaultLiteral()
	left := b.right[u.index()]
	for _, w := range b.dag[u.index()] {
		if b.reaches(u, w) && (w == v || b.reaches(w, v)) && b.left[w.index()] < left {
			left = b.left[w.index()]
			result = w
		}
	}
	if result == DefaultLiteral() {
		return result, ErrSATAssertion
	}
	return result, nil
}

func (b *satBIG) inDeleted(u, v Literal) bool {
	if u.index() > v.index() {
		u, v = v, u
	}
	if int(u.index()) >= len(b.deleted) {
		return false
	}
	for _, w := range b.deleted[u.index()] {
		if w == v {
			return true
		}
	}
	return false
}

func (b *satBIG) addDeleted(u, v Literal) {
	if u.index() > v.index() {
		u, v = v, u
	}
	for len(b.deleted) < len(b.dag) {
		b.deleted = append(b.deleted, nil)
	}
	b.deleted[u.index()] = append(b.deleted[u.index()], v)
}

func (b *satBIG) safeReach(u, v Literal) (bool, error) {
	if !b.reaches(u, v) {
		return false, nil
	}
	for u != v {
		w, err := b.next(u, v)
		if err != nil {
			return false, err
		}
		if b.inDeleted(u.Negated(), w) || w == v.Negated() {
			return false, nil
		}
		u = w
	}
	return true, nil
}
