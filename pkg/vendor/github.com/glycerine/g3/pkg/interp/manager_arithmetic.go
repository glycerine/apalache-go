package interp

// Source checkpoint: src/interp/iz3mgr.cpp linear_comb, sum_inequalities, and mk_idiv.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (m *Manager) SimplifyInequality(inequality ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	op, err := m.Operator(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op != OperatorLE && op != OperatorLT && op != OperatorGE && op != OperatorGT {
		return ast.ASTHandle{}, NewError("not an inequality")
	}
	left, err := m.Arg(inequality, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := m.Arg(inequality, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	simplified, err := m.Simplify(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.ast.DelRef(&simplified)) }()
	right, err = m.ast.RefGet(simplified)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.Make(op, left, right)
}

func (m *Manager) LinearCombination(p, coefficient, q ast.ASTHandle, roundOff bool) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	qNot, qBody, err := m.ast.IsNot(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !qNot {
		qBody = q
	}
	qOp, err := m.Operator(qBody)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if qOp != OperatorLE && qOp != OperatorGE && qOp != OperatorLT && qOp != OperatorGT {
		return ast.ASTHandle{}, NewError("not an inequality")
	}
	qLeft, err := m.Arg(qBody, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	qRight, err := m.Arg(qBody, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var qRHS ast.ASTHandle
	qStrict := false
	if qNot {
		switch qOp {
		case OperatorGT:
			qRHS, err = m.Make(OperatorSub, qRight, qLeft)
		case OperatorLT:
			qRHS, err = m.Make(OperatorSub, qLeft, qRight)
		case OperatorGE:
			qRHS, err = m.Make(OperatorSub, qRight, qLeft)
			qStrict = true
		case OperatorLE:
			qRHS, err = m.Make(OperatorSub, qLeft, qRight)
			qStrict = true
		default:
			return ast.ASTHandle{}, NewError("not an inequality")
		}
	} else {
		switch qOp {
		case OperatorLE:
			qRHS, err = m.Make(OperatorSub, qRight, qLeft)
		case OperatorGE:
			qRHS, err = m.Make(OperatorSub, qLeft, qRight)
		case OperatorLT:
			qRHS, err = m.Make(OperatorSub, qRight, qLeft)
			qStrict = true
		case OperatorGT:
			qRHS, err = m.Make(OperatorSub, qLeft, qRight)
			qStrict = true
		default:
			return ast.ASTHandle{}, NewError("not an inequality")
		}
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pOp, err := m.Operator(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if pOp != OperatorLE && pOp != OperatorLT {
		return ast.ASTHandle{}, NewError("not an inequality")
	}
	pStrict := pOp == OperatorLT
	if roundOff {
		sort, e := m.Type(qRHS)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		integer, e := m.IntType()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		roundOff = sort == integer
	}
	coefficientValue, coefficientIsNumeral, err := m.Numeral(coefficient)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	coefficientIsOne := coefficientIsNumeral && coefficientValue.Cmp(big.NewRat(1, 1)) == 0
	if qStrict && roundOff && (pStrict || !coefficientIsOne) {
		one, e := m.MakeIntText("1")
		if e != nil {
			return ast.ASTHandle{}, e
		}
		qRHS, err = m.Make(OperatorSub, qRHS, one)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		qStrict = false
	}
	qRHS, err = m.Make(OperatorTimes, coefficient, qRHS)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pLeft, err := m.Arg(p, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pRight, err := m.Arg(p, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := m.Make(OperatorPlus, pRight, qRHS)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	resultOp := OperatorLE
	if pStrict || qStrict {
		resultOp = OperatorLT
	}
	return m.Make(resultOp, pLeft, right)
}

func (m *Manager) SumInequalities(coefficients, inequalities []ast.ASTHandle, roundOff bool) (ast.ASTHandle, error) {
	if len(coefficients) != len(inequalities) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	zero, err := m.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := m.Make(OperatorLE, zero, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := range inequalities {
		result, err = m.LinearCombination(result, coefficients[i], inequalities[i], roundOff)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return m.SimplifyInequality(result)
}

func (m *Manager) splitIntegerDivision(term ast.ASTHandle, divisor *big.Int, whole, fraction ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, error) {
	op, err := m.Operator(term)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if op == OperatorPlus {
		count, e := m.NumArgs(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		for i := uint32(0); i < count; i++ {
			arg, e := m.Arg(term, i)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, e
			}
			whole, fraction, e = m.splitIntegerDivision(arg, divisor, whole, fraction)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, e
			}
		}
		return whole, fraction, nil
	}
	if op == OperatorTimes {
		coefficientTerm, e := m.Arg(term, 0)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		coefficient, matched, e := m.Numeral(coefficientTerm)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		if matched && coefficient.IsInt() {
			numerator := coefficient.Num()
			gcd := new(big.Int).GCD(nil, nil, numerator, divisor)
			absoluteDivisor := new(big.Int).Abs(new(big.Int).Set(divisor))
			if gcd.Cmp(absoluteDivisor) == 0 {
				quotient := new(big.Int).Quo(new(big.Int).Set(numerator), divisor)
				quotientTerm, e := m.MakeInt(new(big.Rat).SetInt(quotient))
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				variable, e := m.Arg(term, 1)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				product, e := m.Make(OperatorTimes, quotientTerm, variable)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				whole, e = m.Make(OperatorPlus, whole, product)
				return whole, fraction, e
			}
		}
	}
	fraction, err = m.Make(OperatorPlus, fraction, term)
	return whole, fraction, err
}

func (m *Manager) MakeIDivRational(term ast.ASTHandle, divisor *big.Rat) (result ast.ASTRef, resultErr error) {
	if divisor == nil || divisor.Sign() == 0 || !divisor.IsInt() {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	simplified, err := m.Simplify(term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if divisor.Cmp(big.NewRat(1, 1)) == 0 {
		return simplified, nil
	}
	defer func() { resultErr = errors.Join(resultErr, m.ast.DelRef(&simplified)) }()
	term, err = m.ast.RefGet(simplified)
	if err != nil {
		return ast.ASTRef{}, err
	}
	zero, err := m.MakeIntText("0")
	if err != nil {
		return ast.ASTRef{}, err
	}
	whole, fraction, err := m.splitIntegerDivision(term, divisor.Num(), zero, zero)
	if err != nil {
		return ast.ASTRef{}, err
	}
	fractionRef, err := m.Simplify(fraction)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.ast.DelRef(&fractionRef)) }()
	fraction, err = m.ast.RefGet(fractionRef)
	if err != nil {
		return ast.ASTRef{}, err
	}
	divisorTerm, err := m.MakeInt(divisor)
	if err != nil {
		return ast.ASTRef{}, err
	}
	division, err := m.Make(OperatorIDiv, fraction, divisorTerm)
	if err != nil {
		return ast.ASTRef{}, err
	}
	sum, err := m.Make(OperatorPlus, whole, division)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.Simplify(sum)
}

func (m *Manager) MakeIDiv(term, divisor ast.ASTHandle) (ast.ASTRef, error) {
	value, matched, err := m.Numeral(divisor)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if matched {
		return m.MakeIDivRational(term, value)
	}
	division, err := m.Make(OperatorIDiv, term, divisor)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.ast.NewRef(division)
}
