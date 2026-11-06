package lp

// Source: src/util/lp/permutation_matrix.h/.hpp, retained mpq/mpq and mpq/impq instances.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalPermutation owns separate exact T/X work buffers in a borrowed manager.
// Dense inputs own their numerals; assignments deep-copy them. After a failed
// numerical call, Close releases only this owner's acquired call roots and
// buffers. The caller still owns every completed/partial input coordinate.
// Do not shallow-copy this owner or resize its embedded mapping directly.
type RationalPermutation struct {
	scratch *exactPermutationScratch
	Permutation
	manager *numeral.RationalManager
	tBuffer *IndexedRationalVector
	xBuffer *IndexedRationalVector
}

func NewRationalPermutation(m *numeral.RationalManager, size uint32) *RationalPermutation {
	return &RationalPermutation{Permutation: *NewPermutation(size), manager: m,
		tBuffer: NewIndexedRationalVector(m, size), xBuffer: NewIndexedRationalVector(m, size)}
}

func (p *RationalPermutation) ready() error {
	if p.scratch != nil {
		return ErrExactVectorPending
	}
	if p.tBuffer.calls.pending() || p.xBuffer.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (p *RationalPermutation) Init(size uint32) error {
	if err := p.ready(); err != nil {
		return err
	}
	resizeExactPermutationMappings(&p.Permutation, size)
	if err := p.tBuffer.resizeData(size); err != nil {
		return err
	}
	if err := p.xBuffer.resizeData(size); err != nil {
		return err
	}
	for i := 0; i < len(p.values); i++ {
		p.values[i] = uint32(i)
		p.reverse[i] = uint32(i)
	}
	return nil
}

func (p *RationalPermutation) Resize(size uint32) error {
	if err := p.ready(); err != nil {
		return err
	}
	old := len(p.values)
	resizeExactPermutationMappings(&p.Permutation, size)
	if err := p.tBuffer.resizeData(size); err != nil {
		return err
	}
	if err := p.xBuffer.resizeData(size); err != nil {
		return err
	}

	for i := old; i < len(p.values); i++ {
		p.values[i] = uint32(i)
		p.reverse[i] = uint32(i)
	}
	return nil
}

func (p *RationalPermutation) Close() error {
	if p.scratch != nil {
		if err := p.scratch.close(); err != nil {
			return err
		}
		p.scratch = nil
	}
	if err := p.xBuffer.Close(); err != nil {
		return err
	}
	return p.tBuffer.Close()
}
func (p *RationalPermutation) ApplyLeft(w []numeral.Rational) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.xBuffer.Data) != len(w) {
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
		if err := p.xBuffer.copySlot(&p.xBuffer.Data[i], &w[j]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.xBuffer.copySlot(&w[i], &p.xBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *RationalPermutation) applyRight(w []numeral.Rational) error {
	if len(p.tBuffer.Data) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := 0; i < len(p.reverse); i++ {
		j := p.reverse[i]
		if uint64(j) >= uint64(len(w)) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&p.tBuffer.Data[i], &w[j]); err != nil {
			return err
		}
	}
	for i := 0; i < len(p.reverse); i++ {
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&w[i], &p.tBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *RationalPermutation) ApplyReverseLeftToT(w []numeral.Rational) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.tBuffer.Data) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		j := p.values[i]
		if i >= len(w) || uint64(j) >= uint64(len(p.tBuffer.Data)) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&p.tBuffer.Data[j], &w[i]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&w[i], &p.tBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *RationalPermutation) ApplyReverseLeftToX(w []numeral.Rational) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.xBuffer.Data) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		j := p.values[i]
		if i >= len(w) || uint64(j) >= uint64(len(p.xBuffer.Data)) {
			return ErrBounds
		}
		if err := p.xBuffer.copySlot(&p.xBuffer.Data[j], &w[i]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.xBuffer.copySlot(&w[i], &p.xBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *RationalPermutation) ApplyReverseRightToT(w []numeral.Rational) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.tBuffer.Data) != len(w) {
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
		if err := p.tBuffer.copySlot(&p.tBuffer.Data[i], &w[j]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&w[i], &p.tBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *RationalPermutation) ApplyReverseRightToX(w []numeral.Rational) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.xBuffer.Data) != len(w) {
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
		if err := p.xBuffer.copySlot(&p.xBuffer.Data[i], &w[j]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.xBuffer.copySlot(&w[i], &p.xBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}

// PairPermutation owns separate exact T/X work buffers in a borrowed manager.
// Dense inputs own their numerals; assignments deep-copy them. After a failed
// numerical call, Close releases only this owner's acquired call roots and
// buffers. The caller still owns every completed/partial input coordinate.
// Do not shallow-copy this owner or resize its embedded mapping directly.
type PairPermutation struct {
	scratch *exactPermutationScratch
	Permutation
	manager *numeral.RationalManager
	tBuffer *IndexedRationalVector
	xBuffer *IndexedPairVector
}

func NewPairPermutation(m *numeral.RationalManager, size uint32) *PairPermutation {
	return &PairPermutation{Permutation: *NewPermutation(size), manager: m,
		tBuffer: NewIndexedRationalVector(m, size), xBuffer: NewIndexedPairVector(m, size)}
}

func (p *PairPermutation) ready() error {
	if p.scratch != nil {
		return ErrExactVectorPending
	}
	if p.tBuffer.calls.pending() || p.xBuffer.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (p *PairPermutation) Init(size uint32) error {
	if err := p.ready(); err != nil {
		return err
	}
	resizeExactPermutationMappings(&p.Permutation, size)
	if err := p.tBuffer.resizeData(size); err != nil {
		return err
	}
	if err := p.xBuffer.resizeData(size); err != nil {
		return err
	}
	for i := 0; i < len(p.values); i++ {
		p.values[i] = uint32(i)
		p.reverse[i] = uint32(i)
	}
	return nil
}

func (p *PairPermutation) Resize(size uint32) error {
	if err := p.ready(); err != nil {
		return err
	}
	old := len(p.values)
	resizeExactPermutationMappings(&p.Permutation, size)
	if err := p.tBuffer.resizeData(size); err != nil {
		return err
	}
	if err := p.xBuffer.resizeData(size); err != nil {
		return err
	}

	for i := old; i < len(p.values); i++ {
		p.values[i] = uint32(i)
		p.reverse[i] = uint32(i)
	}
	return nil
}

func (p *PairPermutation) Close() error {
	if p.scratch != nil {
		if err := p.scratch.close(); err != nil {
			return err
		}
		p.scratch = nil
	}
	if err := p.xBuffer.Close(); err != nil {
		return err
	}
	return p.tBuffer.Close()
}
func (p *PairPermutation) ApplyLeft(w []RationalPair) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.xBuffer.Data) != len(w) {
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
		if err := p.xBuffer.copySlot(&p.xBuffer.Data[i], &w[j]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.xBuffer.copySlot(&w[i], &p.xBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *PairPermutation) applyRight(w []numeral.Rational) error {
	if len(p.tBuffer.Data) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := 0; i < len(p.reverse); i++ {
		j := p.reverse[i]
		if uint64(j) >= uint64(len(w)) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&p.tBuffer.Data[i], &w[j]); err != nil {
			return err
		}
	}
	for i := 0; i < len(p.reverse); i++ {
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&w[i], &p.tBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *PairPermutation) ApplyReverseLeftToT(w []numeral.Rational) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.tBuffer.Data) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		j := p.values[i]
		if i >= len(w) || uint64(j) >= uint64(len(p.tBuffer.Data)) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&p.tBuffer.Data[j], &w[i]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&w[i], &p.tBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *PairPermutation) ApplyReverseLeftToX(w []RationalPair) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.xBuffer.Data) != len(w) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		j := p.values[i]
		if i >= len(w) || uint64(j) >= uint64(len(p.xBuffer.Data)) {
			return ErrBounds
		}
		if err := p.xBuffer.copySlot(&p.xBuffer.Data[j], &w[i]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.xBuffer.copySlot(&w[i], &p.xBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *PairPermutation) ApplyReverseRightToT(w []numeral.Rational) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.tBuffer.Data) != len(w) {
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
		if err := p.tBuffer.copySlot(&p.tBuffer.Data[i], &w[j]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.tBuffer.copySlot(&w[i], &p.tBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (p *PairPermutation) ApplyReverseRightToX(w []RationalPair) error {
	if err := p.ready(); err != nil {
		return err
	}
	if len(p.xBuffer.Data) != len(w) {
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
		if err := p.xBuffer.copySlot(&p.xBuffer.Data[i], &w[j]); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		if i >= len(w) {
			return ErrBounds
		}
		if err := p.xBuffer.copySlot(&w[i], &p.xBuffer.Data[i]); err != nil {
			return err
		}
	}
	return nil
}

func (p *RationalPermutation) ApplyRight(w []numeral.Rational) error {
	if err := p.ready(); err != nil {
		return err
	}
	return p.applyRight(w)
}
func (p *RationalPermutation) ApplyLeftIndexed(w *IndexedRationalVector) error {
	if err := p.ready(); err != nil {
		return err
	}
	if err := exactPermutationInput(p.manager, w); err != nil {
		return err
	}
	p.scratch = newExactPermutationScratch(p.manager, uint32(len(w.Index)))
	if err := exactPermutationLeft(&p.Permutation, w, p.scratch, false); err != nil {
		return err
	}
	if err := p.scratch.close(); err != nil {
		return err
	}
	p.scratch = nil
	return nil
}
func (p *RationalPermutation) ApplyReverseLeftIndexed(w *IndexedRationalVector) error {
	if err := p.ready(); err != nil {
		return err
	}
	if err := exactPermutationInput(p.manager, w); err != nil {
		return err
	}
	p.scratch = newExactPermutationScratch(p.manager, uint32(len(w.Index)))
	if err := exactPermutationLeft(&p.Permutation, w, p.scratch, true); err != nil {
		return err
	}
	if err := p.scratch.close(); err != nil {
		return err
	}
	p.scratch = nil
	return nil
}
func (p *RationalPermutation) ApplyRightIndexed(w *IndexedRationalVector) error {
	if err := p.ready(); err != nil {
		return err
	}
	if err := exactPermutationInput(p.manager, w); err != nil {
		return err
	}
	p.scratch = newExactPermutationScratch(p.manager, uint32(len(w.Index)))
	if sourceDebugEnabled {
		p.scratch.debug = NewIndexedRationalVector(p.manager, w.DataSize())
		for i := range w.Data {
			if err := p.scratch.debug.copySlot(&p.scratch.debug.Data[i], &w.Data[i]); err != nil {
				return err
			}
		}
		if err := p.applyRight(p.scratch.debug.Data); err != nil {
			return err
		}
	}
	if err := exactPermutationRight(&p.Permutation, w, p.scratch, false); err != nil {
		return err
	}
	if sourceDebugEnabled {
		equal, err := p.scratch.equal(w)
		if err != nil {
			return err
		}
		if !equal {
			return sourceAssertionFailure()
		}
	}
	if err := p.scratch.close(); err != nil {
		return err
	}
	p.scratch = nil
	return nil
}
func (p *RationalPermutation) ApplyReverseRightIndexed(w *IndexedRationalVector) error {
	if err := p.ready(); err != nil {
		return err
	}
	if err := exactPermutationInput(p.manager, w); err != nil {
		return err
	}
	p.scratch = newExactPermutationScratch(p.manager, uint32(len(w.Index)))
	if err := exactPermutationRight(&p.Permutation, w, p.scratch, true); err != nil {
		return err
	}
	if err := p.scratch.close(); err != nil {
		return err
	}
	p.scratch = nil
	return nil
}

func (p *PairPermutation) ApplyRight(w []numeral.Rational) error {
	if err := p.ready(); err != nil {
		return err
	}
	return p.applyRight(w)
}
func (p *PairPermutation) ApplyLeftIndexed(w *IndexedRationalVector) error {
	if err := p.ready(); err != nil {
		return err
	}
	if err := exactPermutationInput(p.manager, w); err != nil {
		return err
	}
	p.scratch = newExactPermutationScratch(p.manager, uint32(len(w.Index)))
	if err := exactPermutationLeft(&p.Permutation, w, p.scratch, false); err != nil {
		return err
	}
	if err := p.scratch.close(); err != nil {
		return err
	}
	p.scratch = nil
	return nil
}
func (p *PairPermutation) ApplyReverseLeftIndexed(w *IndexedRationalVector) error {
	if err := p.ready(); err != nil {
		return err
	}
	if err := exactPermutationInput(p.manager, w); err != nil {
		return err
	}
	p.scratch = newExactPermutationScratch(p.manager, uint32(len(w.Index)))
	if err := exactPermutationLeft(&p.Permutation, w, p.scratch, true); err != nil {
		return err
	}
	if err := p.scratch.close(); err != nil {
		return err
	}
	p.scratch = nil
	return nil
}
func (p *PairPermutation) ApplyRightIndexed(w *IndexedRationalVector) error {
	if err := p.ready(); err != nil {
		return err
	}
	if err := exactPermutationInput(p.manager, w); err != nil {
		return err
	}
	p.scratch = newExactPermutationScratch(p.manager, uint32(len(w.Index)))
	if sourceDebugEnabled {
		p.scratch.debug = NewIndexedRationalVector(p.manager, w.DataSize())
		for i := range w.Data {
			if err := p.scratch.debug.copySlot(&p.scratch.debug.Data[i], &w.Data[i]); err != nil {
				return err
			}
		}
		if err := p.applyRight(p.scratch.debug.Data); err != nil {
			return err
		}
	}
	if err := exactPermutationRight(&p.Permutation, w, p.scratch, false); err != nil {
		return err
	}
	if sourceDebugEnabled {
		equal, err := p.scratch.equal(w)
		if err != nil {
			return err
		}
		if !equal {
			return sourceAssertionFailure()
		}
	}
	if err := p.scratch.close(); err != nil {
		return err
	}
	p.scratch = nil
	return nil
}
func (p *PairPermutation) ApplyReverseRightIndexed(w *IndexedRationalVector) error {
	if err := p.ready(); err != nil {
		return err
	}
	if err := exactPermutationInput(p.manager, w); err != nil {
		return err
	}
	p.scratch = newExactPermutationScratch(p.manager, uint32(len(w.Index)))
	if err := exactPermutationRight(&p.Permutation, w, p.scratch, true); err != nil {
		return err
	}
	if err := p.scratch.close(); err != nil {
		return err
	}
	p.scratch = nil
	return nil
}

// Each sparse application owns real lexical numeral copies. A stopped call
// retains these concrete owners until Close; it never infers ownership from a
// manager registry or takes another caller's stopped arithmetic invocation.
type exactPermutationScratch struct {
	values, debug, difference *IndexedRationalVector
	indices                   []uint32
}

func newExactPermutationScratch(m *numeral.RationalManager, size uint32) *exactPermutationScratch {
	return &exactPermutationScratch{values: NewIndexedRationalVector(m, size), indices: make([]uint32, size)}
}
func (s *exactPermutationScratch) close() error {
	if s.difference != nil {
		if err := s.difference.Close(); err != nil {
			return err
		}
		s.difference = nil
	}
	if err := s.values.Close(); err != nil {
		return err
	}
	if s.debug != nil {
		if err := s.debug.Close(); err != nil {
			return err
		}
		s.debug = nil
	}
	return nil
}
func exactPermutationInput(m *numeral.RationalManager, w *IndexedRationalVector) error {
	if m != w.manager {
		return ErrExactVectorOwner
	}
	if w.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func exactPermutationLeft(p *Permutation, w *IndexedRationalVector, s *exactPermutationScratch, inverse bool) error {
	for i := len(w.Index); i > 0; {
		i--
		j := w.Index[i]
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if err := s.values.copySlot(&s.values.Data[i], &w.Data[j]); err != nil {
			return err
		}
		s.indices[i] = j
	}
	for i := len(w.Index); i > 0; {
		i--
		j := w.Index[i]
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if err := w.resetSlot(&w.Data[j]); err != nil {
			return err
		}
	}
	for i := len(s.indices); i > 0; {
		i--
		var j uint32
		var err error
		if inverse {
			j, err = p.Value(s.indices[i])
		} else {
			j, err = p.Reverse(s.indices[i])
		}
		if err != nil {
			return err
		}
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if err := w.copySlot(&w.Data[j], &s.values.Data[i]); err != nil {
			return err
		}
		w.Index[i] = j
	}
	return nil
}
func exactPermutationRight(p *Permutation, w *IndexedRationalVector, s *exactPermutationScratch, inverse bool) error {
	copy(s.indices, w.Index)
	for i, j := range w.Index {
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if err := s.values.copySlot(&s.values.Data[i], &w.Data[j]); err != nil {
			return err
		}
	}
	if err := w.Clear(); err != nil {
		return err
	}
	for i, j := range s.indices {
		var mapped uint32
		var err error
		if inverse {
			mapped, err = p.Reverse(j)
		} else {
			mapped, err = p.Value(j)
		}
		if err != nil {
			return err
		}
		if err := w.SetValue(&s.values.Data[i], mapped); err != nil {
			return err
		}
	}
	return nil
}

// Source: lp_settings.hpp:94-103, reached by permutation_matrix.hpp assertions.
func (s *exactPermutationScratch) equal(w *IndexedRationalVector) (bool, error) {
	if len(s.debug.Data) != len(w.Data) {
		return false, nil
	}
	s.difference = NewIndexedRationalVector(w.manager, 2)
	temporary, out := &s.difference.Data[0], &s.difference.Data[1]
	for i := range w.Data {
		before, err := s.difference.calls.begin(w.manager)
		if err != nil {
			return false, err
		}
		err = etaRationalDifference(w.manager, temporary, out, &s.debug.Data[i], &w.Data[i])
		if err = s.difference.calls.after(w.manager, before, err); err != nil {
			return false, err
		}
		zero, err := w.manager.IsZero(out)
		if err != nil {
			return false, err
		}
		// The subtraction's returned temporary dies before the next element or
		// early false return, independently of the containing debug snapshot.
		if err = w.manager.Del(out); err != nil {
			return false, err
		}
		*out = numeral.NewRational()
		if !zero {
			return false, nil
		}
	}
	return true, nil
}

// Buffer destructors precede identity-tail publication in the source. Keep
// mapping resize separate from that final publication for exact numeric owners.
func resizeExactPermutationMappings(p *Permutation, size uint32) {
	if uint64(size) <= uint64(len(p.values)) {
		p.values = p.values[:size]
		p.reverse = p.reverse[:size]
	} else {
		p.values = append(p.values, make([]uint32, int(size)-len(p.values))...)
		p.reverse = append(p.reverse, make([]uint32, int(size)-len(p.reverse))...)
	}
}
