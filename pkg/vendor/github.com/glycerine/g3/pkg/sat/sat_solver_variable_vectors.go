package sat

import "github.com/glycerine/g3/pkg/arena"

type satVariableCharColumn struct {
	data []int8
}

func (c *satVariableCharColumn) extend(n uint32) {
	if n <= uint32(len(c.data)) {
		return
	}
	old := len(c.data)
	c.data = append(c.data, make([]int8, int(n)-old)...)
}
func (c *satVariableCharColumn) read(i uint32) (int8, error) {
	if uint64(i) >= uint64(len(c.data)) {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	return c.data[i], nil
}
func (c *satVariableCharColumn) write(i uint32, v int8) error {
	if uint64(i) >= uint64(len(c.data)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	c.data[i] = v
	return nil
}
func (c *satVariableCharColumn) finalize() {
	c.data = nil
}
