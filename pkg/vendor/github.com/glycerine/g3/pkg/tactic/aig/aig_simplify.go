package aig

// Source: src/tactic/aig/aig.cpp:207-416 (USE_TWO_LEVEL_RULES enabled).

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func opposite(a, b Literal) bool { return a.node == b.node && a.inverted != b.inverted }
func invert(a Literal) Literal   { a.inverted = !a.inverted; return a }

// And applies the source's two-level local simplifications and canonical
// child-ID ordering before interning a retained AND node.
func (m *Manager) And(left, right *Ref) (*Ref, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	l, e := left.literal(m)
	if e != nil {
		return nil, e
	}
	r, e := right.literal(m)
	if e != nil {
		return nil, e
	}
	if m.activeRefs == math.MaxUint32 {
		return nil, arena.ErrBounds
	}
	result, e := m.mkAnd(l, r)
	if e != nil {
		return nil, e
	}
	return m.newRef(result)
}

// mkAnd is the source mk_and wrapper around mk_node, including its TRACE.
func (m *Manager) mkAnd(l, r Literal) (Literal, error) {
	result, err := m.mkNode(l, r)
	if err != nil {
		return Literal{}, err
	}
	if err = m.traceMkAnd(l, r, result); err != nil {
		if result.node.and && result.node.refs == 0 {
			// A newly interned gate has no owning reference yet. Release its
			// child references when trace formatting stops before newRef.
			result.node.refs = 1
			m.dec(result)
			err = errors.Join(err, m.drain())
		}
		return Literal{}, err
	}
	return result, nil
}

func (m *Manager) mkNode(l, r Literal) (Literal, error) {
	f := Literal{node: m.trueNode, inverted: true}
	for {
		n1, n2 := l.node, r.node
		s1, s2 := l.inverted, r.inverted
		if n1.id == 0 {
			if s1 {
				return f, nil
			}
			return r, nil
		}
		if n2.id == 0 {
			if s2 {
				return f, nil
			}
			return l, nil
		}
		if n1 == n2 {
			if s1 == s2 {
				return l, nil
			}
			return f, nil
		}
		if n1.and {
			a, b := n1.children[0], n1.children[1]
			if opposite(a, r) || opposite(b, r) {
				if s1 {
					return r, nil
				}
				return f, nil
			}
			if a == r {
				if s1 {
					l = invert(b)
					continue
				}
				return l, nil
			}
			if b == r {
				if s1 {
					l = invert(a)
					continue
				}
				return l, nil
			}
			if n2.and {
				c, d := n2.children[0], n2.children[1]
				conflict := opposite(a, c) || opposite(a, d) || opposite(b, c) || opposite(b, d)
				switch {
				case !s1 && !s2:
					if conflict {
						return f, nil
					}
					if a == c || b == c {
						r = d
						continue
					}
					if b == c || b == d {
						l = a
						continue
					}
					if a == d || b == d {
						r = c
						continue
					}
					if a == c || a == d {
						l = b
						continue
					}
				case s1 && !s2:
					if conflict {
						return r, nil
					}
					if b == c || b == d {
						l = invert(a)
						continue
					}
					if a == c || a == d {
						l = invert(b)
						continue
					}
				case !s1 && s2:
					if conflict {
						return l, nil
					}
					if c == a || c == b {
						r = invert(d)
						continue
					}
					if d == a || d == b {
						r = invert(c)
						continue
					}
				case s1 && s2:
					if a == c && opposite(b, d) {
						return invert(a), nil
					}
					if opposite(a, c) && b == d {
						return invert(b), nil
					}
				}
			}
		}
		if n2.and {
			a, b := n2.children[0], n2.children[1]
			if opposite(l, a) || opposite(l, b) {
				if s2 {
					return l, nil
				}
				return f, nil
			}
			if a == l {
				if s2 {
					r = invert(b)
					continue
				}
				return r, nil
			}
			if b == l {
				if s2 {
					r = invert(a)
					continue
				}
				return r, nil
			}
		}
		if n1.id > n2.id {
			return m.internCore(r, l)
		}
		return m.internCore(l, r)
	}
}
