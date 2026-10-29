package lp

// Source: src/util/lp/indexed_vector.h/.hpp. Concrete live LU instantiations.

// IndexedFloatVector has the source public dense data and ordered sparse index.
// Matrix algorithms may edit Data and Index directly; RestoreIndex rebuilds the
// index from data. Zero value is empty. Do not shallow-copy after first use.
type IndexedFloatVector struct {
	Data  []float64
	Index []uint32
}

func NewIndexedFloatVector(size uint32) *IndexedFloatVector {
	return &IndexedFloatVector{Data: make([]float64, size)}
}
func (v *IndexedFloatVector) DataSize() uint32 { return uint32(len(v.Data)) }
func (v *IndexedFloatVector) Size() uint32     { return uint32(len(v.Index)) }
func (v *IndexedFloatVector) Clear() error {
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		v.Data[i] = 0
	}
	v.Index = v.Index[:0]
	return nil
}
func (v *IndexedFloatVector) ClearAll() { clear(v.Data); v.Index = v.Index[:0] }
func (v *IndexedFloatVector) Resize(size uint32) error {
	if err := v.Clear(); err != nil {
		return err
	}
	if uint64(size) <= uint64(len(v.Data)) {
		v.Data = v.Data[:size]
	} else {
		v.Data = append(v.Data, make([]float64, int(size)-len(v.Data))...)
	}
	return nil
}
func (v *IndexedFloatVector) Assign(y *IndexedFloatVector) error {
	// Source clears old indexed data before reading y, including self assignment.
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		v.Data[i] = 0
	}
	v.Index = append(v.Index[:0], y.Index...)
	n := len(y.Data)
	if n <= len(v.Data) {
		v.Data = v.Data[:n]
	} else {
		v.Data = append(v.Data, make([]float64, n-len(v.Data))...)
	}
	for _, i := range v.Index {
		if uint64(i) >= uint64(n) {
			return ErrBounds
		}
		v.Data[i] = y.Data[i]
	}
	return nil
}

func (v *IndexedFloatVector) SetValue(value float64, index uint32) error {
	if uint64(index) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	v.Data[index] = value
	for _, i := range v.Index {
		if i == index {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
			break
		}
	}
	v.Index = append(v.Index, index)
	return nil
}
func (v *IndexedFloatVector) EraseFromIndex(j uint32) {
	for k, i := range v.Index {
		if i == j {
			copy(v.Index[k:], v.Index[k+1:])
			v.Index = v.Index[:len(v.Index)-1]
			return
		}
	}
}
func (v *IndexedFloatVector) Add(j uint32, value float64) error {
	if uint64(j) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	wasZero := v.Data[j] == 0
	v.Data[j] += value
	if v.Data[j] == 0 {
		if !wasZero {
			v.EraseFromIndex(j)
		}
	} else if wasZero {
		v.Index = append(v.Index, j)
	}
	return nil
}
func (v *IndexedFloatVector) AddWithDropTolerance(j uint32, value float64) error {
	if uint64(j) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	wasZero := v.Data[j] == 0
	v.Data[j] += value
	x := v.Data[j]
	if indexedFloatSmall(x) {
		v.Data[j] = 0
		if !wasZero {
			v.EraseFromIndex(j)
		}
	} else if wasZero {
		v.Index = append(v.Index, j)
	}
	return nil
}
func indexedFloatSmall(v float64) bool { return v < 1e-14 && v > -1e-14 }
func (v *IndexedFloatVector) CleanUp() error {
	var indexCopy []uint32
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		x := v.Data[i]
		if !indexedFloatSmall(x) {
			indexCopy = append(indexCopy, i)
		} else if x != 0 {
			v.Data[i] = 0
		}
	}
	v.Index = indexCopy
	return nil
}
func (v *IndexedFloatVector) RestoreIndex() {
	v.Index = v.Index[:0]
	for i, x := range v.Data {
		if indexedFloatSmall(x) {
			v.Data[i] = 0
		} else {
			v.Index = append(v.Index, uint32(i))
		}
	}
}

// IsOK preserves the upstream debug body, whose first statement returns true.
func (v *IndexedFloatVector) IsOK() bool { return true }

// IndexedUintVector stores the unsigned markers used by LU row refinement.
// The retained source uses clear, resize, set_value and public data/index reads.
// Zero value is empty. Do not shallow-copy after first use.
type IndexedUintVector struct {
	Data  []uint32
	Index []uint32
}

func NewIndexedUintVector(size uint32) *IndexedUintVector {
	return &IndexedUintVector{Data: make([]uint32, size)}
}
func (v *IndexedUintVector) DataSize() uint32 { return uint32(len(v.Data)) }
func (v *IndexedUintVector) Size() uint32     { return uint32(len(v.Index)) }
func (v *IndexedUintVector) Clear() error {
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(v.Data)) {
			return ErrBounds
		}
		v.Data[i] = 0
	}
	v.Index = v.Index[:0]
	return nil
}
func (v *IndexedUintVector) Resize(size uint32) error {
	if err := v.Clear(); err != nil {
		return err
	}
	if uint64(size) <= uint64(len(v.Data)) {
		v.Data = v.Data[:size]
	} else {
		v.Data = append(v.Data, make([]uint32, int(size)-len(v.Data))...)
	}
	return nil
}
func (v *IndexedUintVector) SetValue(value uint32, index uint32) error {
	if uint64(index) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	v.Data[index] = value
	for _, i := range v.Index {
		if i == index {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
			break
		}
	}
	v.Index = append(v.Index, index)
	return nil
}

// IsOK preserves the upstream debug body, whose first statement returns true.
func (v *IndexedUintVector) IsOK() bool { return true }
