package lp

// Source: src/util/lp/lu.h/.hpp, retained precise T and scalar/pair X owners.

// RationalLU owns an exact factorization in the source matrix's borrowed
// manager. Close every partial constructor result, including failed results.
// Do not shallow-copy; RHS numerals remain owned by their caller on every path.
type RationalLU struct {
	*exactLUState
	Q, R, RWave *RationalPermutation
	Tail        []RationalTailMatrix
}

func NewRationalLU(a *RationalStaticMatrix, basis []uint32, s *Settings) (*RationalLU, error) {
	n := a.RowCount()
	m := &RationalLU{exactLUState: &exactLUState{Dim: n, A: a, Settings: s}, Q: NewRationalPermutation(a.manager, n), R: NewRationalPermutation(a.manager, n), RWave: NewRationalPermutation(a.manager, n)}
	m.q, m.r = &m.Q.Permutation, &m.R.Permutation
	err := m.initialize(basis)
	for _, tail := range m.tails {
		if tail.eta != nil {
			m.Tail = append(m.Tail, tail.eta)
		} else {
			m.Tail = append(m.Tail, tail.dense)
		}
	}
	m.tails = nil
	return m, err
}

func (m *RationalLU) Close() (err error) {
	if m.closed {
		return nil
	}
	defer m.finish(&err)
	if err = m.closeUpdatePending(); err != nil {
		return err
	}
	for len(m.Tail) > 0 {
		if err = m.Tail[0].Close(); err != nil {
			return err
		}
		m.Tail = m.Tail[1:]
	}
	if err = m.closeMembers(); err != nil {
		return err
	}
	for _, p := range []*RationalPermutation{m.RWave, m.R, m.Q} {
		if err = p.Close(); err != nil {
			return err
		}
	}
	m.closed = true
	return nil
}

// PairLU shares only rational coefficient algorithms with RationalLU. Its
// X permutations and tail applications own concrete rational-pair scratch.
type PairLU struct {
	*exactLUState
	Q, R, RWave *PairPermutation
	Tail        []PairTailMatrix
}

func NewPairLU(a *RationalStaticMatrix, basis []uint32, s *Settings) (*PairLU, error) {
	n := a.RowCount()
	m := &PairLU{exactLUState: &exactLUState{Dim: n, A: a, Settings: s}, Q: NewPairPermutation(a.manager, n), R: NewPairPermutation(a.manager, n), RWave: NewPairPermutation(a.manager, n)}
	m.q, m.r = &m.Q.Permutation, &m.R.Permutation
	err := m.initialize(basis)
	// Transfer each coefficient owner into its actual concrete pair tail, also
	// on failure. No numerical copy or new allocation occurs in this binding.
	for _, tail := range m.tails {
		if tail.eta != nil {
			m.Tail = append(m.Tail, &PairEtaMatrix{RationalEtaMatrix: tail.eta})
		} else {
			m.Tail = append(m.Tail, &PairSquareDenseSubmatrix{RationalSquareDenseSubmatrix: tail.dense})
		}
	}
	m.tails = nil
	return m, err
}

func (m *PairLU) Close() (err error) {
	if m.closed {
		return nil
	}
	defer m.finish(&err)
	if err = m.closeUpdatePending(); err != nil {
		return err
	}
	for len(m.Tail) > 0 {
		if err = m.Tail[0].Close(); err != nil {
			return err
		}
		m.Tail = m.Tail[1:]
	}
	if err = m.closeMembers(); err != nil {
		return err
	}
	for _, p := range []*PairPermutation{m.RWave, m.R, m.Q} {
		if err = p.Close(); err != nil {
			return err
		}
	}
	m.closed = true
	return nil
}
