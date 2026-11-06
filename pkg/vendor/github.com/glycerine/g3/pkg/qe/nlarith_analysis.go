package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

type NLLiteralAnalysis struct {
	manager  *ast.ASTManager
	Variable ast.ASTRef
	Literals []*NLLiteral
	closed   bool
}

func (u *NonlinearArithmeticUtil) Analyze(variable *ast.ContainsApp, literals []ast.ASTHandle) (*NLLiteralAnalysis, bool, error) {
	if err := u.open(); err != nil {
		return nil, false, err
	}
	if variable == nil {
		return nil, false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return nil, false, err
	}
	vr, err := u.manager.NewRef(target)
	if err != nil {
		return nil, false, err
	}
	a := &NLLiteralAnalysis{manager: u.manager, Variable: vr}
	for _, literal := range literals {
		contains, cause := variable.Check(literal)
		if cause != nil {
			_ = a.Close()
			return nil, false, cause
		}
		if !contains {
			continue
		}
		parsed, ok, cause := u.ParseLiteral(variable, literal)
		if cause != nil || !ok {
			_ = a.Close()
			return nil, ok, cause
		}
		a.Literals = append(a.Literals, parsed)
	}
	return a, true, nil
}
func (a *NLLiteralAnalysis) MaxDegree() (uint32, error) {
	if a == nil || a.closed {
		return 0, ast.ErrASTHandle
	}
	var result uint32
	for _, l := range a.Literals {
		d, err := l.Polynomial.Degree()
		if err != nil {
			return 0, err
		}
		if d > result {
			result = d
		}
	}
	return result, nil
}
func (a *NLLiteralAnalysis) IsLinear() (bool, error) {
	if a == nil || a.closed {
		return false, ast.ErrASTHandle
	}
	for _, literal := range a.Literals {
		degree, err := literal.Polynomial.Degree()
		if err != nil {
			return false, err
		}
		if degree > 1 {
			return false, nil
		}
		if degree == 1 {
			coefficient, err := literal.Polynomial.Coefficient(1)
			if err != nil {
				return false, err
			}
			value, numeral, err := a.numericCoefficient(coefficient)
			if err != nil {
				return false, err
			}
			if !numeral {
				return false, nil
			}
			// Source is_linear rewrites the leading coefficient through
			// mk_numeral(rational, false), normalizing it to a Real numeral.
			normalized, err := literal.Polynomial.manager.ArithmeticRealNumeralText(value.RatString())
			if err != nil {
				return false, err
			}
			if err = literal.Polynomial.setCoefficient(1, normalized); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

// numericCoefficient follows nlarith_util::imp::is_numeral, which folds
// numeric add, multiply, subtract, and unary-minus coefficient expressions.
func (a *NLLiteralAnalysis) numericCoefficient(h ast.ASTHandle) (*big.Rat, bool, error) {
	text, _, numeral, err := a.manager.ArithmeticNumeralText(h)
	if err != nil || numeral {
		if err != nil {
			return nil, false, err
		}
		value, ok := new(big.Rat).SetString(text)
		if !ok {
			return nil, false, ast.ErrASTDomain
		}
		return value, true, nil
	}
	family, err := a.manager.GetFamilyID(h)
	if err != nil {
		return nil, false, err
	}
	arithmetic, err := a.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, false, err
	}
	if family != arithmetic {
		return nil, false, nil
	}
	kind, err := a.manager.GetDeclKind(h)
	if err != nil {
		return nil, false, err
	}
	count, err := a.manager.GetNumArgs(h)
	if err != nil {
		return nil, false, err
	}
	if kind != 6 && kind != 7 && kind != 8 && kind != 9 {
		return nil, false, nil
	}
	if count == 0 {
		return nil, false, ast.ErrASTDomain
	}
	first, err := a.manager.GetArg(h, 0)
	if err != nil {
		return nil, false, err
	}
	result, ok, err := a.numericCoefficient(first)
	if err != nil || !ok {
		return nil, ok, err
	}
	if kind == 8 {
		return new(big.Rat).Neg(result), true, nil
	}
	for i := uint32(1); i < count; i++ {
		arg, cause := a.manager.GetArg(h, uint32(i))
		if cause != nil {
			return nil, false, cause
		}
		value, numeric, cause := a.numericCoefficient(arg)
		if cause != nil || !numeric {
			return nil, numeric, cause
		}
		switch kind {
		case 6:
			result.Add(result, value)
		case 7:
			result.Sub(result, value)
		case 9:
			result.Mul(result, value)
		}
	}
	return result, true, nil
}

func (a *NLLiteralAnalysis) HasSingleQuadratic() (uint32, bool, error) {
	if a == nil || a.closed {
		return 0, false, ast.ErrASTHandle
	}
	found := uint32(0)
	seen := false
	for i, l := range a.Literals {
		d, err := l.Polynomial.Degree()
		if err != nil {
			return 0, false, err
		}
		if d > 2 {
			return 0, false, nil
		}
		if d == 2 {
			if l.Comparison == NLComparisonEQ {
				return 0, false, nil
			}
			if seen {
				return 0, false, nil
			}
			seen = true
			found = uint32(i)
		}
	}
	return found, seen, nil
}
func (a *NLLiteralAnalysis) Close() (result error) {
	if a == nil || a.manager == nil {
		return ast.ErrASTHandle
	}
	if a.closed {
		return nil
	}
	for i := len(a.Literals) - 1; i >= 0; i-- {
		result = errors.Join(result, a.Literals[i].Close())
	}
	result = errors.Join(result, a.manager.DelRef(&a.Variable))
	if result == nil {
		a.closed = true
		a.Literals = nil
	}
	return result
}
