//go:build !g3checked

package numeral

import (
	"github.com/glycerine/g3/pkg/arena"
)

type NumericCellRef uint32

func MakeNumericRef(slot arena.Slot, _ arena.Generation) NumericCellRef { return NumericCellRef(slot) }
func NumericSlot(ref NumericCellRef) arena.Slot                         { return arena.Slot(ref) }
func ValidNumericRef(s *numericCellSlab, ref NumericCellRef) bool {
	_, e := s.Read(NumericSlot(ref))
	return e == nil
}
