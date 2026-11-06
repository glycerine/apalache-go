package aig

// Source: src/tactic/aig/aig.cpp:647-724.

// iteShape recognizes the source's four complementary-child positions in an
// AND node whose two children are inverted AND nodes. Returned literals are
// borrowed from the node graph and remain valid while the caller owns it.
func iteShape(n *node) (bool, Literal, Literal, Literal) {
	if n == nil || !n.and {
		return false, Literal{}, Literal{}, Literal{}
	}
	l, r := n.children[0], n.children[1]
	if !l.inverted || !r.inverted || !l.node.and || !r.node.and {
		return false, Literal{}, Literal{}, Literal{}
	}
	l1, l2 := l.node.children[0], l.node.children[1]
	r1, r2 := r.node.children[0], r.node.children[1]
	if opposite(l1, r1) {
		l1, l2, r1, r2 = invert(l1), invert(l2), invert(r1), invert(r2)
		if l1.inverted {
			return true, r1, l2, r2
		}
		return true, l1, r2, l2
	}
	if opposite(l1, r2) {
		l1, l2, r1, r2 = invert(l1), invert(l2), invert(r1), invert(r2)
		if l1.inverted {
			return true, r2, l2, r1
		}
		return true, l1, r1, l2
	}
	if opposite(l2, r1) {
		l1, l2, r1, r2 = invert(l1), invert(l2), invert(r1), invert(r2)
		if l2.inverted {
			return true, r1, l1, r2
		}
		return true, l2, r2, l1
	}
	if opposite(l2, r2) {
		l1, l2, r1, r2 = invert(l1), invert(l2), invert(r1), invert(r2)
		if l2.inverted {
			return true, r2, l1, r1
		}
		return true, l2, r1, l1
	}
	return false, Literal{}, Literal{}, Literal{}
}
