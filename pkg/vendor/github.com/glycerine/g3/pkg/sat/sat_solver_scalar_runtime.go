package sat

// Concrete native unsigned work columns for retained SAT consumers.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type satUintColumn struct {
	self    *satUintColumn
	runtime *umemory.Runtime
	values  []uint32
	closed  bool
}

func (c *satUintColumn) Bind(r *umemory.Runtime) { *c = satUintColumn{runtime: r}; c.self = c }
func (c *satUintColumn) open() error {
	if c == nil || c.self != c || c.runtime == nil {
		return ErrSATOwner
	}
	if c.closed {
		return arena.ErrClosed
	}
	return c.runtime.CheckOpen()
}
func (c *satUintColumn) Size() (uint32, error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	return uint32(len(c.values)), nil
}
func (c *satUintColumn) Shrink(n uint32) error {
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
func (c *satUintColumn) Finalize() error {
	if c == nil || c.self != c {
		return ErrSATOwner
	}
	c.values = nil
	c.closed = true
	return nil
}
func (c *satUintColumn) Push(v uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	if uint64(len(c.values)) >= uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	c.values = append(c.values, v)
	return nil
}
func (c *satUintColumn) Read(i uint32) (uint32, error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	if uint64(i) >= uint64(len(c.values)) {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	return c.values[i], nil
}
func (c *satUintColumn) Write(i, v uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(c.values)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	c.values[i] = v
	return nil
}
func (c *satUintColumn) copyFrom(src *satUintColumn) error {
	if e := c.open(); e != nil {
		return e
	}
	if e := src.open(); e != nil {
		return e
	}
	if c.runtime != src.runtime {
		return ErrSATOwner
	}
	if c != src {
		c.values = append(c.values[:0], src.values...)
	}
	return nil
}
func (c *satUintColumn) resize(n uint32, value uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	old := len(c.values)
	if int(n) <= old {
		c.values = c.values[:n]
		return nil
	}
	c.values = append(c.values, make([]uint32, int(n)-old)...)
	for i := old; i < len(c.values); i++ {
		c.values[i] = value
	}
	return nil
}
