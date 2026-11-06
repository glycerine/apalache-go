package sat

func satLUTValue(table uint64, args []bool) bool {
	var idx uint32
	for i, x := range args {
		if x {
			idx |= 1 << uint32(i)
		}
	}
	return table&(uint64(1)<<idx) != 0
}
