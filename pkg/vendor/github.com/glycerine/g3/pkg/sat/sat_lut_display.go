package sat

func satLUTMaskString(mask uint64, size uint32) string {
	b := make([]byte, size)
	for i := uint32(0); i < size; i++ {
		b[i] = '0' + byte(mask>>i&1)
	}
	return string(b)
}
