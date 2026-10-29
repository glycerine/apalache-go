//go:build g3checked

package numeral

import "github.com/glycerine/g3/pkg/arena"

type NumericCellRef uint64

func MakeNumericRef(slot arena.Slot, generation arena.Generation) NumericCellRef {
	return NumericCellRef(uint64(generation)<<32 | uint64(slot))
}
func NumericSlot(ref NumericCellRef) arena.Slot { return arena.Slot(uint32(ref)) }
func ValidNumericRef(s *numericCellSlab, ref NumericCellRef) bool {
	return s.IsLive(NumericSlot(ref), arena.Generation(uint64(ref)>>32))
}
