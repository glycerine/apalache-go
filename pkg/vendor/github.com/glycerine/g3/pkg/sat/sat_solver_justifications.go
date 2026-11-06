package sat

import "github.com/glycerine/g3/pkg/arena"

// Native values retain the exact checked clause locator alongside the source
// kind/literal words. The surrounding variable-state header owns this member;
// native backing has no Runtime allocation or separate provenance reservation.
type satSolverJustifications struct {
	self    *satSolverJustifications
	clauses *ClauseAllocator // fixed non-owning allocator binding
	values  []satJustification
	closed  bool
}

func (c *satSolverJustifications) bind() { c.self = c }
func (c *satSolverJustifications) open() error {
	if c == nil || c.self != c {
		return ErrSATOwner
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}
func (c *satSolverJustifications) bindClauses(a *ClauseAllocator) error {
	if e := c.open(); e != nil {
		return e
	}
	if e := a.open(); e != nil {
		return e
	}
	if c.clauses != nil && c.clauses != a {
		return ErrSATOwner
	}
	c.clauses = a
	return nil
}
func (c *satSolverJustifications) size() (uint32, error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	return uint32(len(c.values)), nil
}
func (c *satSolverJustifications) checkIndex(i uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(c.values)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	return nil
}
func (c *satSolverJustifications) checkValue(j satJustification) error {
	if j.Kind() == 3 && (c.clauses == nil || j.locator.owner != c.clauses.origin) {
		return ErrSATOwner
	}
	return nil
}
func (c *satSolverJustifications) read(i uint32) (satJustification, error) {
	if e := c.checkIndex(i); e != nil {
		return satJustification{}, e
	}
	j := c.values[i]
	return j, c.checkValue(j)
}
func (c *satSolverJustifications) write(i uint32, j satJustification) error {
	if e := c.checkIndex(i); e != nil {
		return e
	}
	if e := c.checkValue(j); e != nil {
		return e
	}
	c.values[i] = j
	return nil
}
func (c *satSolverJustifications) push(j satJustification) error {
	if e := c.open(); e != nil {
		return e
	}
	if e := c.checkValue(j); e != nil {
		return e
	}
	if uint64(len(c.values)) >= uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	c.values = append(c.values, j)
	return nil
}
func (c *satSolverJustifications) shrink(n uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	if uint64(n) > uint64(len(c.values)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	c.values = c.values[:n]
	return nil
}
func (c *satSolverJustifications) finalize() error {
	if c == nil || c.self != c {
		return ErrSATOwner
	}
	c.values = nil
	c.closed = true
	return nil
}
