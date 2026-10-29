package lp

// Source: src/util/lp/permutation_matrix.h/.hpp, shared unsigned index algorithms.

// Permutation stores the exact source row-to-column mapping and reverse mapping.
// Set updates one entry and its reverse only; construction may temporarily have
// duplicates. Resize preserves retained entries, including invalidated mappings
// after a shrink. No implicit bijection repair is performed. Do not shallow-copy.
type Permutation struct {
	values, reverse, work []uint32
}

func NewPermutation(size uint32) *Permutation {
	p := &Permutation{}
	p.Init(size)
	return p
}
func PermutationFromValues(size uint32, values []uint32) (*Permutation, error) {
	p := &Permutation{values: make([]uint32, size), reverse: make([]uint32, size)}
	for i := uint32(0); i < size; i++ {
		if uint64(i) >= uint64(len(values)) {
			return p, ErrBounds
		}
		if err := p.Set(i, values[i]); err != nil {
			return p, err
		}
	}
	return p, nil
}
func (p *Permutation) Size() uint32     { return uint32(len(p.reverse)) }
func (p *Permutation) Values() []uint32 { return p.values } // borrowed read-only
func (p *Permutation) Value(i uint32) (uint32, error) {
	if uint64(i) >= uint64(len(p.values)) {
		return 0, ErrBounds
	}
	return p.values[i], nil
}
func (p *Permutation) Reverse(i uint32) (uint32, error) {
	if uint64(i) >= uint64(len(p.reverse)) {
		return 0, ErrBounds
	}
	return p.reverse[i], nil
}
func (p *Permutation) Set(i, value uint32) error {
	if uint64(i) >= uint64(len(p.values)) || uint64(value) >= uint64(len(p.reverse)) {
		return ErrBounds
	}
	p.values[i] = value
	p.reverse[value] = i
	return nil
}
func (p *Permutation) Init(size uint32) {
	p.Resize(size)
	for i := uint32(0); i < size; i++ {
		p.values[i] = i
		p.reverse[i] = i
	}
}
func (p *Permutation) Resize(size uint32) {
	old := len(p.values)
	if uint64(size) <= uint64(old) {
		p.values = p.values[:size]
		p.reverse = p.reverse[:size]
	} else {
		p.values = append(p.values, make([]uint32, int(size)-old)...)
		p.reverse = append(p.reverse, make([]uint32, int(size)-old)...)
	}
	for i := old; i < len(p.values); i++ {
		p.values[i] = uint32(i)
		p.reverse[i] = uint32(i)
	}
}
func (p *Permutation) IsIdentity() bool {
	for i := len(p.reverse); i > 0; {
		i--
		if p.values[i] != uint32(i) {
			return false
		}
	}
	return true
}
func (p *Permutation) IsDense() bool { return false }
func (p *Permutation) Entry(i, j uint32) (uint32, error) {
	if uint64(i) >= uint64(len(p.values)) {
		return 0, ErrBounds
	}
	if p.values[i] == j {
		return 1, nil
	}
	return 0, nil
}
func (p *Permutation) TransposeLeft(i, j uint32) error {
	if uint64(i) >= uint64(len(p.reverse)) || uint64(j) >= uint64(len(p.reverse)) {
		return ErrBounds
	}
	if i == j {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	pi, pj := p.reverse[i], p.reverse[j]
	if err := p.Set(pi, j); err != nil {
		return err
	}
	return p.Set(pj, i)
}
func (p *Permutation) TransposeRight(i, j uint32) error {
	if uint64(i) >= uint64(len(p.values)) || uint64(j) >= uint64(len(p.values)) {
		return ErrBounds
	}
	if i == j {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	pi, pj := p.values[i], p.values[j]
	if err := p.Set(i, pj); err != nil {
		return err
	}
	return p.Set(j, pi)
}
func (p *Permutation) MultiplyLeft(q *Permutation) error {
	p.work = append(p.work[:0], p.values...)
	if p.Size() != q.Size() {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		qi, err := q.Value(uint32(i))
		if err != nil {
			return err
		}
		if uint64(qi) >= uint64(len(p.work)) {
			return ErrBounds
		}
		if err := p.Set(uint32(i), p.work[qi]); err != nil {
			return err
		}
	}
	return nil
}
func (p *Permutation) MultiplyRight(q *Permutation) error {
	p.work = append(p.work[:0], p.values...)
	if p.Size() != q.Size() {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := len(p.reverse); i > 0; {
		i--
		qi, err := q.Value(p.work[i])
		if err != nil {
			return err
		}
		if err := p.Set(uint32(i), qi); err != nil {
			return err
		}
	}
	return nil
}
func (p *Permutation) MultiplyReverseRight(q *Permutation) error {
	if p.Size() != q.Size() {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	p.work = append(p.work[:0], p.values...)
	for i := len(p.reverse); i > 0; {
		i--
		qi, err := q.Reverse(p.work[i])
		if err != nil {
			return err
		}
		if err := p.Set(uint32(i), qi); err != nil {
			return err
		}
	}
	return nil
}
func (p *Permutation) MultiplyReverseLeft(q *Permutation) error {
	p.work = append(p.work[:0], p.values...)
	// Upstream has no size assertion in this particular multiplication body.
	for i := len(p.reverse); i > 0; {
		i--
		qi, err := q.Reverse(uint32(i))
		if err != nil {
			return err
		}
		if uint64(qi) >= uint64(len(p.work)) {
			return ErrBounds
		}
		if err := p.Set(uint32(i), p.work[qi]); err != nil {
			return err
		}
	}
	return nil
}
