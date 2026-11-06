package arith

// Source: src/tactic/arith/fm_tactic.cpp:1360-1401.
func fmLiteralVariable(l int32) uint32 {
	if l < 0 {
		return uint32(-int64(l))
	}
	return uint32(l)
}

func fmMergeLiterals(lower, upper []int32) ([]int32, bool) {
	result := append([]int32(nil), lower...)
	signs := make(map[uint32]int8, len(lower))
	for _, l := range lower {
		s := int8(1)
		if l < 0 {
			s = -1
		}
		signs[fmLiteralVariable(l)] = s
	}
	for _, l := range upper {
		s := int8(1)
		if l < 0 {
			s = -1
		}
		p := fmLiteralVariable(l)
		if previous := signs[p]; previous != 0 {
			if previous != s {
				return nil, true
			}
			continue
		}
		// Source marks only the lower literals; repeated upper-only literals
		// are appended without adding them to that scratch sign table.
		result = append(result, l)
	}
	return result, false
}
