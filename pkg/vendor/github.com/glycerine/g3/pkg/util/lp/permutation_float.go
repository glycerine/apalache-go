package lp

// Source: src/util/lp/permutation_matrix.h/.hpp, concrete double,double methods.

import "math"

// FloatPermutation owns the source's two distinct dense work buffers. Its
// embedded Permutation contains the shared unsigned project algorithms. Use
// this owner's Init/Resize to resize its numeric buffers along with the mapping.
type FloatPermutation struct {
	Permutation
	tBuffer, xBuffer []float64
}

func NewFloatPermutation(size uint32) *FloatPermutation {
	p := &FloatPermutation{}
	p.Init(size)
	return p
}
func FloatPermutationFromValues(size uint32, values []uint32) (*FloatPermutation, error) {
	indices, err := PermutationFromValues(size, values)
	return &FloatPermutation{Permutation: *indices, tBuffer: make([]float64, size), xBuffer: make([]float64, size)}, err
}
func (p *FloatPermutation) Init(size uint32) {
	p.Permutation.Init(size)
	p.resizeBuffers(size)
}
func (p *FloatPermutation) Resize(size uint32) {
	p.Permutation.Resize(size)
	p.resizeBuffers(size)
}
func (p *FloatPermutation) resizeBuffers(size uint32) {
	if uint64(size) <= uint64(len(p.tBuffer)) {
		p.tBuffer = p.tBuffer[:size]
	} else {
		p.tBuffer = append(p.tBuffer, make([]float64, int(size)-len(p.tBuffer))...)
	}
	if uint64(size) <= uint64(len(p.xBuffer)) {
		p.xBuffer = p.xBuffer[:size]
	} else {
		p.xBuffer = append(p.xBuffer, make([]float64, int(size)-len(p.xBuffer))...)
	}
}
func (p *FloatPermutation) Inverse() (*FloatPermutation, error) {
	return FloatPermutationFromValues(p.Size(), p.reverse)
}
func (p *FloatPermutation) RowCount() uint32    { return p.Size() }
func (p *FloatPermutation) ColumnCount() uint32 { return p.Size() }
func (p *FloatPermutation) Element(i, j uint32) (float64, error) {
	v, err := p.Entry(i, j)
	return float64(v), err
}
func (p *FloatPermutation) ApplyLeft(w []float64) error {
	if len(p.xBuffer) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		j := p.values[i]
		if uint64(j) >= uint64(len(w)) {
			return ErrBounds
		}
		p.xBuffer[i] = w[j]
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		w[i] = p.xBuffer[i]
	}
	return nil
}
func (p *FloatPermutation) ApplyRight(w []float64) error {
	if len(p.tBuffer) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := 0; i < len(p.reverse); i++ {
		j := p.reverse[i]
		if uint64(j) >= uint64(len(w)) {
			return ErrBounds
		}
		p.tBuffer[i] = w[j]
	}
	for i := 0; i < len(p.reverse); i++ {
		if i >= len(w) {
			return ErrBounds
		}
		w[i] = p.tBuffer[i]
	}
	return nil
}
func (p *FloatPermutation) ApplyReverseLeftToT(w []float64) error {
	if len(p.tBuffer) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		j := p.values[i]
		if i >= len(w) || uint64(j) >= uint64(len(p.tBuffer)) {
			return ErrBounds
		}
		p.tBuffer[j] = w[i]
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		w[i] = p.tBuffer[i]
	}
	return nil
}
func (p *FloatPermutation) ApplyReverseLeftToX(w []float64) error {
	if len(p.xBuffer) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		j := p.values[i]
		if i >= len(w) || uint64(j) >= uint64(len(p.xBuffer)) {
			return ErrBounds
		}
		p.xBuffer[j] = w[i]
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		w[i] = p.xBuffer[i]
	}
	return nil
}
func (p *FloatPermutation) ApplyReverseRightToT(w []float64) error {
	if len(p.tBuffer) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		j := p.values[i]
		if uint64(j) >= uint64(len(w)) {
			return ErrBounds
		}
		p.tBuffer[i] = w[j]
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		w[i] = p.tBuffer[i]
	}
	return nil
}
func (p *FloatPermutation) ApplyReverseRightToX(w []float64) error {
	if len(p.xBuffer) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		j := p.values[i]
		if uint64(j) >= uint64(len(w)) {
			return ErrBounds
		}
		p.xBuffer[i] = w[j]
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		w[i] = p.xBuffer[i]
	}
	return nil
}

// These sparse applications preserve the source copy, clear and index order.
func (p *FloatPermutation) ApplyLeftIndexed(w *IndexedFloatVector) error {
	values, indices, err := floatPermutationCopyAside(w)
	if err != nil {
		return err
	}
	if err := floatPermutationClearData(w); err != nil {
		return err
	}
	for i := len(values); i > 0; {
		i--
		j, err := p.Reverse(indices[i])
		if err != nil {
			return err
		}
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		w.Data[j] = values[i]
		w.Index[i] = j
	}
	return nil
}
func (p *FloatPermutation) ApplyReverseLeftIndexed(w *IndexedFloatVector) error {
	values, indices, err := floatPermutationCopyAside(w)
	if err != nil {
		return err
	}
	if err := floatPermutationClearData(w); err != nil {
		return err
	}
	for i := len(values); i > 0; {
		i--
		j, err := p.Value(indices[i])
		if err != nil {
			return err
		}
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		w.Data[j] = values[i]
		w.Index[i] = j
	}
	return nil
}
func floatPermutationCopyAside(w *IndexedFloatVector) ([]float64, []uint32, error) {
	values := make([]float64, len(w.Index))
	indices := make([]uint32, len(w.Index))
	for i := len(values); i > 0; {
		i--
		j := w.Index[i]
		if uint64(j) >= uint64(len(w.Data)) {
			return values, indices, ErrBounds
		}
		values[i] = w.Data[j]
		indices[i] = j
	}
	return values, indices, nil
}
func floatPermutationClearData(w *IndexedFloatVector) error {
	for i := len(w.Index); i > 0; {
		i--
		j := w.Index[i]
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		w.Data[j] = 0
	}
	return nil
}
func (p *FloatPermutation) ApplyRightIndexed(w *IndexedFloatVector) error {
	var debugCopy []float64
	if sourceDebugEnabled {
		debugCopy = append([]float64(nil), w.Data...)
		if err := p.ApplyRight(debugCopy); err != nil {
			return err
		}
	}
	values := make([]float64, len(w.Index))
	indices := append([]uint32(nil), w.Index...)
	for i, j := range w.Index {
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		values[i] = w.Data[j]
	}
	if err := w.Clear(); err != nil {
		return err
	}
	for i, j := range indices {
		mapped, err := p.Value(j)
		if err != nil {
			return err
		}
		if err := w.SetValue(values[i], mapped); err != nil {
			return err
		}
	}
	if sourceDebugEnabled && !floatPermutationVectorsEqual(debugCopy, w.Data) {
		return sourceAssertionFailure()
	}
	return nil
}
func (p *FloatPermutation) ApplyReverseRightIndexed(w *IndexedFloatVector) error {
	values := make([]float64, 0, len(w.Index))
	indices := append([]uint32(nil), w.Index...)
	for _, j := range w.Index {
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		values = append(values, w.Data[j])
	}
	if err := w.Clear(); err != nil {
		return err
	}
	for i, j := range indices {
		mapped, err := p.Reverse(j)
		if err != nil {
			return err
		}
		if err := w.SetValue(values[i], mapped); err != nil {
			return err
		}
	}
	return nil
}
func floatPermutationVectorsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, da := range a {
		db := b[i]
		amax := math.Max(math.Abs(da), math.Abs(db))
		if amax > 1 {
			da /= amax
			db /= amax
		}
		if math.Abs(da-db) > 1e-6 {
			return false
		}
	}
	return true
}
