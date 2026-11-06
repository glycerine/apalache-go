package polynomial

import "github.com/glycerine/g3/pkg/arena"

// Extend native pivot indices, preserving the retained prefix and filling only
// new entries. Berlekamp uses -1 as the unassigned row/column sentinel.
func upolyFactorInt32Resize(values *[]int32, n uint32, value int32) error {
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	old := len(*values)
	if int(n) <= old {
		*values = (*values)[:n]
		return nil
	}
	*values = append(*values, make([]int32, int(n)-old)...)
	for i := old; i < int(n); i++ {
		(*values)[i] = value
	}
	return nil
}
