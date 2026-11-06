package rel

// Source: src/muz/rel/dl_sparse_table.{h,cpp} column_layout.
import (
	"fmt"
	"math/bits"
)

type sparseColumn struct{ offset, length uint32 }
type SparseLayout struct {
	columns                     []sparseColumn
	entryBytes, functionalBytes uint32
}

func sparseDomainBits(n uint64) uint32 {
	if n == 0 {
		return 1
	}
	if n <= uint64(^uint32(0)) {
		if n == 1 {
			return 1
		}
		return uint32(bits.Len32(uint32(n - 1)))
	}
	// Match get_domain_length's split into a low 32-bit word and a high
	// word. Its unary-high-word branch intentionally allocates 33 bits for
	// the exact 2^32 domain boundary.
	high := uint32(n >> 32)
	if uint32(n) != 0 && high != ^uint32(0) {
		high++
	}
	if high == 1 {
		return 33
	}
	if high > 0x80000000 {
		return 64
	}
	return 32 + uint32(bits.Len32(high-1))
}
func NewSparseLayout(signature Signature, functional int) (SparseLayout, error) {
	if functional < 0 || functional > signature.Len() {
		return SparseLayout{}, ErrTable
	}
	if signature.Len() == 0 {
		return SparseLayout{}, nil
	}
	cols := make([]sparseColumn, signature.Len())
	var offset uint32
	first := signature.Len() - functional
	for i, d := range signature.domains {
		bits := sparseDomainBits(d)
		if i > 0 && (bits > 54 || i == first) {
			offset = (offset + 7) &^ 7
		}
		cols[i] = sparseColumn{offset, bits}
		offset += bits
	}
	offset = (offset + 7) &^ 7
	layout := SparseLayout{columns: cols, entryBytes: offset / 8}
	if functional > 0 {
		layout.functionalBytes = layout.entryBytes - cols[first].offset/8
	}
	return layout, nil
}
func (l SparseLayout) EntryBytes() uint32 { return l.entryBytes }
func (l SparseLayout) pack(signature Signature, f Fact) ([]byte, error) {
	if len(f) != len(l.columns) {
		return nil, fmt.Errorf("%w: fact arity mismatch", ErrTable)
	}
	out := make([]byte, l.entryBytes)
	for i, v := range f {
		if uint64(v) >= signature.domains[i] {
			return nil, fmt.Errorf("%w: fact value outside column domain", ErrTable)
		}
		c := l.columns[i]
		for bit := uint32(0); bit < c.length; bit++ {
			if (uint64(v)>>bit)&1 != 0 {
				p := c.offset + bit
				out[p/8] |= 1 << (p % 8)
			}
		}
	}
	return out, nil
}
func (l SparseLayout) unpack(data []byte) Fact {
	out := make(Fact, len(l.columns))
	for i, c := range l.columns {
		var v uint64
		for bit := uint32(0); bit < c.length; bit++ {
			p := c.offset + bit
			if data[p/8]&(1<<(p%8)) != 0 {
				v |= 1 << bit
			}
		}
		out[i] = Element(v)
	}
	return out
}
