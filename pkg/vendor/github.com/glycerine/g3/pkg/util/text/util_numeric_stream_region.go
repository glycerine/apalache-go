package text

// Source: src/util/small_object_allocator.cpp:172-242.

import (
	"github.com/glycerine/g3/pkg/arena"
	"math"
)

// AppendCStringRegion inserts the actual borrowed arena bytes as one field.
// It reads through the first NUL before padding and consumes width once.
func (s *NumericTextStream) AppendCStringRegion(region *arena.BytesRegion, ref arena.BytesRegionRef, offset uint32) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if region == nil {
		return arena.ErrInvalidSlot
	}
	var length uint64
	for {
		offset := uint64(offset) + length
		if offset > math.MaxUint32 {
			return arena.ErrBounds
		}
		c, e := region.Read(ref, uint32(offset))
		if e != nil {
			return e
		}
		if c == 0 {
			break
		}
		length++
	}
	if s.FileBad {
		return nil
	}
	pad, e := s.fieldPadding(length)
	if e != nil {
		return e
	}
	if s.state.Alignment != NumericLeft {
		if e = s.padding(pad); e != nil {
			return e
		}
	}
	for i := uint64(0); i < length; i++ {
		c, e := region.Read(ref, uint32(uint64(offset)+i))
		if e != nil {
			return e
		}
		if e = s.emitByte(c); e != nil {
			return e
		}
	}
	if s.state.Alignment == NumericLeft {
		return s.padding(pad)
	}
	return nil
}
