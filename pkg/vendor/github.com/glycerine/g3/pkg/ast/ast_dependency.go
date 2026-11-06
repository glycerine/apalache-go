package ast

// Concrete AST-valued specialization of util/dependency.h.

import (
	"github.com/glycerine/g3/pkg/arena"
)

type ASTDependency struct {
	origin uint64
	record arena.SmallBytesRef
}

func (d ASTDependency) IsNull() bool { return d == ASTDependency{} }

type ASTDependencyJoinProgress struct{ Completed, PendingLeaf ASTDependency }

const astDependencyMaxRC uint32 = (1 << 30) - 1

// Encoded project reference/mark word, padding and value/children.
type astDependencyNode struct {
	rc           uint32
	marked, leaf bool
	value        astNodeRef
	left, right  arena.SmallBytesRef
}
type astDependencyManager struct {
	owner                *ASTManager
	allocator            *arena.SmallBytes
	todo                 []arena.SmallBytesRef
	live                 uint32
	pendingRoot, current arena.SmallBytesRef
	pending              bool
	stage, child         uint8
}

func dependencyNull(r arena.SmallBytesRef) bool { return r == (arena.SmallBytesRef{}) }
func (d *astDependencyManager) valid(h ASTDependency) (arena.SmallBytesRef, error) {
	if h.IsNull() {
		return arena.SmallBytesRef{}, nil
	}
	if h.origin != d.owner.origin || !d.allocator.IsLive(h.record) {
		return arena.SmallBytesRef{}, ErrASTHandle
	}
	if _, e := d.node(h.record); e != nil {
		return arena.SmallBytesRef{}, e
	}
	return h.record, nil
}
func (d *astDependencyManager) external(r arena.SmallBytesRef) ASTDependency {
	if dependencyNull(r) {
		return ASTDependency{}
	}
	return ASTDependency{d.owner.origin, r}
}
func (d *astDependencyManager) node(r arena.SmallBytesRef) (astDependencyNode, error) {
	if !d.allocator.IsLive(r) {
		return astDependencyNode{}, ErrASTHandle
	}
	n, e := d.allocator.Size(r)
	if e != nil || (n != 16 && n != 24) {
		return astDependencyNode{}, ErrASTHandle
	}
	w, e := d.owner.readWord(r, 0)
	if e != nil {
		return astDependencyNode{}, e
	}
	a, e := d.owner.readWord(r, 8)
	if e != nil {
		return astDependencyNode{}, e
	}
	b, e := d.owner.readWord(r, 12)
	if e != nil {
		return astDependencyNode{}, e
	}
	c := astDependencyNode{rc: w & astDependencyMaxRC, marked: w&(1<<30) != 0, leaf: w&(1<<31) != 0}
	if c.leaf {
		if n != 16 {
			return c, ErrASTHandle
		}
		c.value = astNodeRef{a, b}
	} else {
		if n != 24 {
			return c, ErrASTHandle
		}
		c.left = arena.SmallBytesRef{Slot: arena.Slot(a), Generation: arena.Generation(b)}
		a, e = d.owner.readWord(r, 16)
		if e != nil {
			return c, e
		}
		b, e = d.owner.readWord(r, 20)
		if e != nil {
			return c, e
		}
		c.right = arena.SmallBytesRef{Slot: arena.Slot(a), Generation: arena.Generation(b)}
	}
	return c, nil
}

func (d *astDependencyManager) store(r arena.SmallBytesRef, c astDependencyNode) error {
	w := c.rc & astDependencyMaxRC
	if c.marked {
		w |= 1 << 30
	}
	if c.leaf {
		w |= 1 << 31
	}
	if e := d.owner.writeWord(r, 0, w); e != nil {
		return e
	}
	var words [4]uint32
	var count uint32
	if c.leaf {
		words = [4]uint32{c.value.slot, c.value.generation}
		count = 2
	} else {
		words = [4]uint32{uint32(c.left.Slot), uint32(c.left.Generation), uint32(c.right.Slot), uint32(c.right.Generation)}
		count = 4
	}
	for i := uint32(0); i < count; i++ {
		if e := d.owner.writeWord(r, 8+i*4, words[i]); e != nil {
			return e
		}
	}
	return nil
}
func (d *astDependencyManager) inc(r arena.SmallBytesRef) error {
	if d.pending {
		return ErrASTPending
	}
	if dependencyNull(r) {
		return nil
	}
	c, e := d.node(r)
	if e != nil {
		return e
	}
	c.rc = (c.rc + 1) & astDependencyMaxRC
	return d.store(r, c)
}
func (d *astDependencyManager) leaf(v astNodeRef) (arena.SmallBytesRef, error) {
	if d.pending {
		return arena.SmallBytesRef{}, ErrASTPending
	}
	if v.slot == 0 {
		return arena.SmallBytesRef{}, nil
	}
	r, e := d.allocator.Allocate(16)
	if e != nil {
		return r, e
	}
	if e = d.owner.incNode(v); e != nil {
		d.allocator.Deallocate(16, r)
		return arena.SmallBytesRef{}, e
	}
	e = d.store(r, astDependencyNode{leaf: true, value: v})
	if e == nil {
		d.live++
	}
	return r, e
}
func (d *astDependencyManager) join(a, b arena.SmallBytesRef) (arena.SmallBytesRef, error) {
	if d.pending {
		return arena.SmallBytesRef{}, ErrASTPending
	}
	if dependencyNull(a) {
		return b, nil
	}
	if dependencyNull(b) || a == b {
		return a, nil
	}
	r, e := d.allocator.Allocate(24)
	if e != nil {
		return r, e
	}
	if e = d.inc(a); e != nil {
		d.allocator.Deallocate(24, r)
		return arena.SmallBytesRef{}, e
	}
	if e = d.inc(b); e != nil {
		d.allocator.Deallocate(24, r)
		return arena.SmallBytesRef{}, e
	}
	e = d.store(r, astDependencyNode{left: a, right: b})
	if e == nil {
		d.live++
	}
	return r, e
}
func (d *astDependencyManager) dec(r arena.SmallBytesRef) error {
	_, e := d.decConsumed(r)
	return e
}

func (d *astDependencyManager) decConsumed(r arena.SmallBytesRef) (bool, error) {
	if d.pending {
		return false, ErrASTPending
	}
	if dependencyNull(r) {
		return true, nil
	}
	c, e := d.node(r)
	if e != nil {
		return false, e
	}
	if ASTSourceDebug && c.rc == 0 {
		return false, ErrASTDomain
	}
	c.rc = (c.rc - 1) & astDependencyMaxRC
	if e = d.store(r, c); e != nil {
		return false, e
	}
	if c.rc != 0 {
		return true, nil
	}
	return true, d.del(r)
}

// Native scratch preserves left/right enqueue and LIFO destruction order.
func (d *astDependencyManager) del(r arena.SmallBytesRef) error {
	d.pending = true
	d.pendingRoot = r
	d.todo = append(d.todo, r)
	return d.continueDel()
}

// A failed callback can consume its AST reference before it reports an error.
// The current node and child cursor retain the completed source prefix; an
// explicit continuation finishes that prefix without replaying it.
func (d *astDependencyManager) continueDel() error {
	for !dependencyNull(d.current) || len(d.todo) != 0 {
		if dependencyNull(d.current) {
			d.current = d.todo[len(d.todo)-1]
			d.todo = d.todo[:len(d.todo)-1]
			d.stage, d.child = 0, 0
		}
		x := d.current
		c, e := d.node(x)
		if e != nil {
			return e
		}
		bytes := uint32(24)
		if c.leaf {
			bytes = 16
			if d.stage == 0 {
				consumed, next := d.owner.decNode(c.value)
				if consumed {
					d.stage = 1
				}
				if next != nil {
					return next
				}
				d.stage = 1
			}
		} else {
			for d.child < 2 {
				child := c.left
				if d.child == 1 {
					child = c.right
				}
				v, e := d.node(child)
				if e != nil {
					return e
				}
				if ASTSourceDebug && v.rc == 0 {
					return ErrASTDomain
				}
				v.rc = (v.rc - 1) & astDependencyMaxRC
				if e = d.store(child, v); e != nil {
					return e
				}
				if v.rc == 0 {
					d.todo = append(d.todo, child)
				}
				d.child++
			}
		}
		if e = d.allocator.Deallocate(bytes, x); e != nil {
			if !d.allocator.IsLive(x) {
				d.live--
				d.current = arena.SmallBytesRef{}
			}
			return e
		}
		d.live--
		d.current = arena.SmallBytesRef{}
	}
	d.pending = false
	d.pendingRoot = arena.SmallBytesRef{}
	return nil
}
func (d *astDependencyManager) closeAfterNodeTeardown() {
	d.todo = nil
	d.live = 0
	d.pending = false
	d.pendingRoot, d.current = arena.SmallBytesRef{}, arena.SmallBytesRef{}
	d.stage, d.child = 0, 0
}
