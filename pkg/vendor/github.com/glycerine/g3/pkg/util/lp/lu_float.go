package lp

// Source: src/util/lp/lu.h and lu.hpp:125-160,491-571,619-639,717-779.
// Retained double/double initial sparse/dense LU factorization.

type FloatLU struct {
	Dim             uint32
	A               *FloatStaticMatrix // borrowed
	Q, R, RWave     *FloatPermutation
	U               *FloatSparseMatrix
	Dense           *FloatSquareDenseSubmatrix // borrowed after publication into Tail
	Tail            []FloatTailMatrix
	rowEtaWork      *IndexedFloatVector
	wExtension      *IndexedFloatVector
	yCopy           *IndexedFloatVector
	ii              *IndexedUintVector
	Settings        *Settings // borrowed
	Failure         bool
	RefactorCounter uint32
	status          LUStatus
	stopped, closed bool
}

func NewFloatLU(a *FloatStaticMatrix, basis []uint32, s *Settings) (m *FloatLU, err error) {
	n := a.RowCount()
	m = &FloatLU{Dim: n, A: a, Q: NewFloatPermutation(n), R: NewFloatPermutation(n), RWave: NewFloatPermutation(n), Settings: s}
	defer m.finish(&err)
	m.U, err = NewFloatSparseBasisMatrix(a, basis)
	if err != nil {
		return m, err
	}
	m.rowEtaWork = NewIndexedFloatVector(n)
	m.wExtension = NewIndexedFloatVector(0)
	m.yCopy = NewIndexedFloatVector(0)
	m.ii = NewIndexedUintVector(0)
	if err = luDebugBasis(n, a.ColumnCount(), basis); err != nil {
		return m, err
	}
	s.Stats.Factorizations++
	err = m.createInitialFactorization()
	return m, err
}
func (m *FloatLU) ready() error {
	if m.closed {
		return ErrLUClosed
	}
	if m.stopped {
		return ErrLUStopped
	}
	return nil
}
func (m *FloatLU) finish(err *error) {
	if *err != nil {
		m.stopped = true
	}
}
func (m *FloatLU) GetStatus() LUStatus  { return m.status }
func (m *FloatLU) SetStatus(s LUStatus) { m.status = s }
func (m *FloatLU) NeedToRefactor() bool { return m.RefactorCounter >= 200 }
func (m *FloatLU) TooDense(j uint32) bool {
	return luTooDense(m.Dim, j, m.U.ActiveElements(), m.Settings)
}
func (m *FloatLU) swapRows(j, k uint32) error {
	if j == k {
		return nil
	}
	if err := m.Q.TransposeLeft(j, k); err != nil {
		return err
	}
	return m.U.SwapRows(j, k)
}
func (m *FloatLU) swapColumns(j, k uint32) error {
	if j == k {
		return nil
	}
	if err := m.R.TransposeRight(j, k); err != nil {
		return err
	}
	return m.U.SwapColumns(j, k)
}
func (m *FloatLU) pivotRow(row uint32) (bool, error) {
	eta, ok, err := m.U.FillEtaMatrix(row)
	if err != nil {
		return false, err
	}
	if !ok {
		m.status = LUDegenerated
		return false, nil
	}
	if eta == nil {
		// Literal source branch ignores the shortening result.
		_, err = m.U.ShortenActiveMatrix(row, nil)
		return err == nil, err
	}
	ok, err = m.U.PivotWithEta(row, eta, m.Settings)
	if err != nil || !ok {
		return false, err
	}
	if err = eta.Conjugate(&m.Q.Permutation); err != nil {
		return false, err
	}
	m.Tail = append(m.Tail, eta)
	return true, nil
}
func (m *FloatLU) processColumn(j uint32) error {
	var pi, pj uint32
	ok, err := m.U.GetPivotForColumn(&pi, &pj, m.Settings.PartialPivotingConstant, j)
	if err != nil {
		return err
	}
	if !ok || pi == ^uint32(0) {
		m.Failure = true
		return nil
	}
	if err = m.swapColumns(j, pj); err != nil {
		return err
	}
	if err = m.swapRows(j, pi); err != nil {
		return err
	}
	ok, err = m.pivotRow(j)
	if err != nil {
		return err
	}
	if !ok {
		m.Failure = true
	}
	return nil
}
func (m *FloatLU) pivotDense(i uint32) error {
	j, err := m.Dense.FindPivotColumn(i)
	if err != nil {
		return err
	}
	if j == -1 {
		m.Failure = true
		return nil
	}
	if i != uint32(j) {
		if err = m.swapColumns(i, uint32(j)); err != nil {
			return err
		}
		if err = m.Dense.SwapColumns(i, uint32(j)); err != nil {
			return err
		}
	}
	return m.Dense.Pivot(i, m.Settings)
}
func (m *FloatLU) createInitialFactorization() error {
	if err := m.U.PrepareForFactorization(); err != nil {
		return err
	}
	j := uint32(0)
	for ; j < m.Dim; j++ {
		if err := m.processColumn(j); err != nil {
			return err
		}
		if m.Failure {
			m.status = LUDegenerated
			return nil
		}
		if m.TooDense(j) {
			break
		}
	}
	if j == m.Dim {
		return nil
	}
	j++
	var err error
	m.Dense, err = NewFloatSquareDenseSubmatrix(m.U, j)
	if err != nil {
		return err
	}
	for ; j < m.Dim; j++ {
		if err = m.pivotDense(j); err != nil {
			return err
		}
		if m.Failure {
			m.status = LUDegenerated
			return nil
		}
	}
	if err = m.Dense.UpdateParent(m.Settings); err != nil {
		return err
	}
	if sourceDebugEnabled {
		ok, e := m.Dense.IsLMatrix()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
	}
	if err = m.Dense.Conjugate(&m.Q.Permutation); err != nil {
		return err
	}
	m.Tail = append(m.Tail, m.Dense)
	m.RefactorCounter = 0
	return nil
}
func (m *FloatLU) Close() error {
	// Native floating tails have no numerical allocations needing explicit Del.
	m.Tail = nil
	m.Dense = nil
	m.U = nil
	m.Q = nil
	m.R = nil
	m.RWave = nil
	m.rowEtaWork = nil
	m.wExtension = nil
	m.yCopy = nil
	m.ii = nil
	m.closed = true
	return nil
}
