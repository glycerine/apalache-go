package lp

// Source: src/util/lp/lu.h/.hpp. Shared mpq coefficient factorization for
// the retained mpq/mpq and mpq/numeric_pair<mpq> instances.

// Both exact instances have identical coefficient algorithms. Concrete typed
// owners below supply their distinct X permutations and tail applications.
// This state never captures numerical receipts around delegated child calls.
type exactLUState struct {
	Dim                           uint32
	A                             *RationalStaticMatrix // borrowed
	U                             *RationalSparseMatrix
	Dense                         *RationalSquareDenseSubmatrix // borrowed after tail publication
	Settings                      *Settings                     // borrowed
	Failure                       bool
	RefactorCounter               uint32
	status                        LUStatus
	stopped, closed               bool
	q, r                          *Permutation // borrowed from the concrete typed permutations
	rowEtaWork, wExtension, yCopy *IndexedRationalVector
	tails                         []exactLUCoefficientTail // transferred to the typed owner before return
	pendingEta                    *RationalEtaMatrix
	denseOwned                    bool
	updateWork                    *IndexedRationalVector
	pendingRowEta                 *RationalRowEtaMatrix
	pendingDiagonal               *RationalDiagonalMatrix
}
type exactLUCoefficientTail struct {
	eta   *RationalEtaMatrix
	dense *RationalSquareDenseSubmatrix
}

func (m *exactLUState) ready() error {
	if m.closed {
		return ErrLUClosed
	}
	if m.stopped {
		return ErrLUStopped
	}
	return nil
}
func (m *exactLUState) finish(err *error) {
	if *err != nil {
		m.stopped = true
	}
}
func (m *exactLUState) initialize(basis []uint32) (err error) {
	defer m.finish(&err)
	m.U, err = NewRationalSparseBasisMatrix(m.A, basis)
	if err != nil {
		return err
	}
	// These members are constructed only after the U constructor completes.
	m.rowEtaWork = NewIndexedRationalVector(m.A.manager, m.Dim)
	m.wExtension = NewIndexedRationalVector(m.A.manager, 0)
	m.yCopy = NewIndexedRationalVector(m.A.manager, 0)
	if sourceDebugEnabled && m.Settings.UseTableau() {
		return ErrAssertion
	}
	if err = luDebugBasis(m.Dim, m.A.ColumnCount(), basis); err != nil {
		return err
	}
	m.Settings.Stats.Factorizations++
	return m.createInitialFactorization()
}

func (m *exactLUState) GetStatus() LUStatus  { return m.status }
func (m *exactLUState) SetStatus(s LUStatus) { m.status = s }
func (m *exactLUState) NeedToRefactor() bool { return m.RefactorCounter >= 200 }
func (m *exactLUState) TooDense(j uint32) bool {
	return luTooDense(m.Dim, j, m.U.ActiveElements(), m.Settings)
}
func (m *exactLUState) swapRows(j, k uint32) error {
	if j == k {
		return nil
	}
	if err := m.q.TransposeLeft(j, k); err != nil {
		return err
	}
	return m.U.SwapRows(j, k)
}
func (m *exactLUState) swapColumns(j, k uint32) error {
	if j == k {
		return nil
	}
	if err := m.r.TransposeRight(j, k); err != nil {
		return err
	}
	return m.U.SwapColumns(j, k)
}
func (m *exactLUState) pivotRow(row uint32) (bool, error) {
	eta, ok, err := m.U.FillEtaMatrix(row)
	m.pendingEta = eta
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
	if err = eta.Conjugate(m.q); err != nil {
		return false, err
	}
	m.tails = append(m.tails, exactLUCoefficientTail{eta: eta})
	m.pendingEta = nil
	return true, nil
}
func (m *exactLUState) processColumn(j uint32) error {
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
func (m *exactLUState) pivotDense(i uint32) error {
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
func (m *exactLUState) createInitialFactorization() error {
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
	m.Dense, err = NewRationalSquareDenseSubmatrix(m.U, j)
	m.denseOwned = m.Dense != nil
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
	if err = m.Dense.Conjugate(m.q); err != nil {
		return err
	}
	m.tails = append(m.tails, exactLUCoefficientTail{dense: m.Dense})
	m.denseOwned = false
	m.RefactorCounter = 0
	return nil
}

// The typed owner resolves failed update calls and children before tails.
// Initial-factorization children retain failed construction/arithmetic prefixes.
func (m *exactLUState) closeMembers() error {
	if m.updateWork != nil {
		if err := m.updateWork.Close(); err != nil {
			return err
		}
		m.updateWork = nil
	}
	if m.pendingEta != nil {
		if err := m.pendingEta.Close(); err != nil {
			return err
		}
		m.pendingEta = nil
	}
	if m.denseOwned {
		if err := m.Dense.Close(); err != nil {
			return err
		}
		m.denseOwned = false
	}
	for _, v := range []*IndexedRationalVector{m.yCopy, m.wExtension, m.rowEtaWork} {
		if v != nil {
			if err := v.CloseForward(); err != nil {
				return err
			}
		}
	}
	if m.U != nil {
		if err := m.U.Close(); err != nil {
			return err
		}
		m.U = nil
	}
	return nil
}
