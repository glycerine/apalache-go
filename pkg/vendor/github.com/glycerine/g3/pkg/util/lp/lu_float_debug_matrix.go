//go:build g3debug

package lp

// Source: src/util/lp/lu.hpp:640-686,947-958; sparse_matrix.h debug matrix view.

func (m *FloatSparseMatrix) RowCount() uint32                     { return m.Dimension() }
func (m *FloatSparseMatrix) ColumnCount() uint32                  { return m.Dimension() }
func (m *FloatSparseMatrix) Element(i, j uint32) (float64, error) { return m.Get(i, j) }

type floatDebugDiagonalView struct{ *FloatDiagonalMatrix }

func (m floatDebugDiagonalView) Element(i, j uint32) (float64, error) {
	return m.FloatDiagonalMatrix.Element(i, j), nil
}

// debugTail binds the source virtual debug-matrix operations to the retained
// concrete tail types. No any-valued numeric adapter is introduced.
func (m *FloatLU) debugTail(t FloatTailMatrix) (FloatDebugMatrix, error) {
	switch a := t.(type) {
	case *FloatEtaMatrix:
		a.length = m.Dim
		return a, nil
	case *FloatRowEtaMatrix:
		a.length = m.Dim
		return a, nil
	case *FloatDiagonalMatrix:
		a.SetDimension(m.Dim)
		return floatDebugDiagonalView{a}, nil
	case *FloatPermutation:
		return a, nil
	case *FloatSquareDenseSubmatrix:
		return a, nil
	default:
		return nil, ErrBounds
	}
}
func (m *FloatLU) BasisDebugMatrix(basis []uint32) (*FloatDenseMatrix, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	if m.U == nil || uint64(len(basis)) != uint64(m.Dim) || uint64(len(basis)) != uint64(m.U.Dimension()) {
		return nil, ErrAssertion
	}
	result, err := NewFloatDenseMatrix(m.Dim, m.Dim)
	if err != nil {
		return result, err
	}
	for i := uint32(0); i < m.Dim; i++ {
		for j := uint32(0); j < m.Dim; j++ {
			value, err := m.A.Element(i, basis[j])
			if err != nil {
				return result, err
			}
			if err = result.SetElement(i, j, value); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}
func (m *FloatLU) LeftDebugMatrix(basis []uint32) (*FloatDenseMatrix, error) {
	result, err := m.BasisDebugMatrix(basis)
	if err != nil {
		return result, err
	}
	for _, tail := range m.Tail {
		a, err := m.debugTail(tail)
		if err != nil {
			return result, err
		}
		next, err := MultiplyFloatDebugMatrices(a, result)
		if err != nil {
			return next, err
		}
		if err = result.Assign(next); err != nil {
			return result, err
		}
	}
	return result, nil
}
func (m *FloatLU) RightDebugMatrix() (*FloatDenseMatrix, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	if m.U == nil || m.R == nil || m.Q == nil {
		return nil, ErrBounds
	}
	result, err := MultiplyFloatDebugMatrices(m.U, m.R)
	if err != nil {
		return result, err
	}
	next, err := MultiplyFloatDebugMatrices(m.Q, result)
	if err != nil {
		return next, err
	}
	if err = result.Assign(next); err != nil {
		return result, err
	}
	return result, nil
}
func (m *FloatLU) DebugIsCorrect(basis []uint32) (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
	if m.GetStatus() != LUOK {
		return false, nil
	}
	a, err := m.LeftDebugMatrix(basis)
	if err != nil {
		return false, err
	}
	b, err := m.RightDebugMatrix()
	if err != nil {
		return false, err
	}
	return FloatDebugMatricesEqual(a, b)
}
