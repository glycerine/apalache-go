package polynomial

// Source: src/math/polynomial/polynomial.cpp:6159-6201,7405-7407.

// GreaterEqual proves p >= q using the source square-monomial sufficient
// criterion. False means the criterion could not prove the relation; it does
// not mean that a negative witness exists.
func (m *MultivariatePolynomialCore) GreaterEqual(p, q Polynomial) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	leftInfo, err := m.Info(p)
	if err != nil {
		return false, err
	}
	rightInfo, err := m.Info(q)
	if err != nil {
		return false, err
	}
	i, j := uint32(0), uint32(0)
	for i < leftInfo.Size || j < rightInfo.Size {
		if i < leftInfo.Size && j == rightInfo.Size {
			monomial, cause := m.TermMonomial(p, i)
			if cause != nil {
				return false, cause
			}
			monomialInfo, cause := m.monomials.Info(monomial)
			if cause != nil {
				return false, cause
			}
			if monomialInfo.Size == 0 {
				coefficient, cause := m.Coefficient(p, i)
				if cause != nil {
					return false, cause
				}
				positive, cause := m.modular.IsPos(&coefficient)
				if cause != nil || !positive {
					return false, cause
				}
				i++
				continue
			}
			break
		}
		if i == leftInfo.Size && j < rightInfo.Size {
			monomial, cause := m.TermMonomial(q, j)
			if cause != nil {
				return false, cause
			}
			monomialInfo, cause := m.monomials.Info(monomial)
			if cause != nil {
				return false, cause
			}
			if monomialInfo.Size == 0 {
				coefficient, cause := m.Coefficient(q, j)
				if cause != nil {
					return false, cause
				}
				negative, cause := m.modular.IsNeg(&coefficient)
				if cause != nil || !negative {
					return false, cause
				}
				j++
				continue
			}
			break
		}
		if i == leftInfo.Size || j == rightInfo.Size {
			break
		}
		leftMonomial, cause := m.TermMonomial(p, i)
		if cause != nil {
			return false, cause
		}
		rightMonomial, cause := m.TermMonomial(q, j)
		if cause != nil {
			return false, cause
		}
		if leftMonomial != rightMonomial {
			coefficient, cause := m.Coefficient(p, i)
			if cause != nil {
				return false, cause
			}
			positive, cause := m.modular.IsPos(&coefficient)
			if cause != nil {
				return false, cause
			}
			square, cause := m.monomials.IsSquare(leftMonomial)
			if cause != nil {
				return false, cause
			}
			if positive && square {
				i++
				continue
			}
			return false, nil
		}
		leftCoefficient, cause := m.Coefficient(p, i)
		if cause != nil {
			return false, cause
		}
		rightCoefficient, cause := m.Coefficient(q, j)
		if cause != nil {
			return false, cause
		}
		equal, cause := m.modular.Equal(&leftCoefficient, &rightCoefficient)
		if cause != nil {
			return false, cause
		}
		if equal {
			i++
			j++
			continue
		}
		square, cause := m.monomials.IsSquare(leftMonomial)
		if cause != nil {
			return false, cause
		}
		greater, cause := m.modular.GreaterEqual(&leftCoefficient, &rightCoefficient)
		if cause != nil {
			return false, cause
		}
		if !square || !greater {
			return false, nil
		}
		i++
		j++
	}
	return i == leftInfo.Size && j == rightInfo.Size, nil
}
