package numeral

// Source: src/util/mpz.cpp:big_set; src/util/mpf.cpp:327-333.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// A call-local borrowed Int address in sole Runtime storage. It carries no
// acquisition authority and owns no payload or cold header. The three words
// are signed inline value, compressed cell low, compressed cell high.

type integerRuntimeValueReceiver struct {
	runtime *umemory.Runtime
	block   umemory.Block
	offset  uint64
}

func (r *integerRuntimeValueReceiver) validate(m *IntegerManager, a *Int) error {
	if r == nil {
		return nil
	}
	if r.runtime == nil || r.runtime != m.Memory || a == nil {
		return umemory.ErrRuntimeOwner
	}
	extent, e := r.runtime.Size(r.block)
	if e != nil {
		return e
	}
	if r.offset > extent || extent-r.offset < 12 {
		return arena.ErrBounds
	}
	val, e := r.runtime.ReadUint32(r.block, r.offset)
	if e != nil {
		return e
	}
	low, e := r.runtime.ReadUint32(r.block, r.offset+4)
	if e != nil {
		return e
	}
	high, e := r.runtime.ReadUint32(r.block, r.offset+8)
	if e != nil {
		return e
	}
	if int32(val) != a.Val || NumericCellRef(uint64(low)|uint64(high)<<32) != a.Cell {
		return umemory.ErrRuntimeOwner
	}
	return nil
}
func (r *integerRuntimeValueReceiver) publishValue(a *Int) error {
	if r == nil {
		return nil
	}
	return r.runtime.WriteUint32(r.block, r.offset, uint32(a.Val))
}
func (r *integerRuntimeValueReceiver) publishCell(a *Int) error {
	if r == nil {
		return nil
	}
	if e := r.runtime.WriteUint32(r.block, r.offset+4, uint32(a.Cell)); e != nil {
		return e
	}
	return r.runtime.WriteUint32(r.block, r.offset+8, uint32(uint64(a.Cell)>>32))
}
func (m *IntegerManager) copyFromOwnerWithPhysicalReceipt(dst *Int, sourceManager *IntegerManager, src *Int, receipt *integerCopyAllocationReceipt, receiver *integerRuntimeValueReceiver) error {
	return m.copyFromOwnerWithPhysicalAllocationContext(dst, sourceManager, src, receipt, integerAllocationInvocationContext{}, receiver)
}
