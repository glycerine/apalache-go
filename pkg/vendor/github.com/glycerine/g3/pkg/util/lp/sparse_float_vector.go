package lp

// Source: src/util/lp/sparse_vector.h, retained double coefficient storage.

// FloatSparseEntry is one literal sparse-list term. Duplicates and explicit
// zeros remain stored and are applied in insertion order.
type FloatSparseEntry struct {
	Index uint32
	Value float64
}
type SparseFloatVector struct{ Data []FloatSparseEntry }

func (v *SparseFloatVector) Append(index uint32, value float64) {
	v.Data = append(v.Data, FloatSparseEntry{index, value})
}
func (v *SparseFloatVector) Size() uint32 { return uint32(len(v.Data)) }
func (v *SparseFloatVector) At(index uint32) float64 {
	for _, entry := range v.Data {
		if entry.Index == index {
			return entry.Value
		}
	}
	return 0
}
func (v *SparseFloatVector) Divide(value float64) error {
	if FloatEpsilonSmall(value, 1e-12) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := range v.Data {
		v.Data[i].Value /= value
	}
	return nil
}
