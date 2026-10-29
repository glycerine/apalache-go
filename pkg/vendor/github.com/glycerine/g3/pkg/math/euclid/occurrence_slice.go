package euclid

// eraseOccurrenceValue follows the source svector::erase(value): an absent
// value leaves the list unchanged, and only its first occurrence is removed.
func eraseOccurrenceValue(values []uint32, target uint32) []uint32 {
	for i, value := range values {
		if value == target {
			copy(values[i:], values[i+1:])
			values[len(values)-1] = 0
			return values[:len(values)-1]
		}
	}
	return values
}
