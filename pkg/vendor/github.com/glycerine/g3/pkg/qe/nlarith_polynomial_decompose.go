package qe

import "github.com/glycerine/g3/pkg/ast"

func (u *NonlinearArithmeticUtil) zeroOne(witness ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, error) {
	z, err := u.term.zero(witness)
	if err != nil {
		return z, ast.ASTHandle{}, err
	}
	integer, err := u.term.isInteger(witness)
	if err != nil {
		return z, ast.ASTHandle{}, err
	}
	if integer {
		o, err := u.manager.ArithmeticIntegerNumeralText("1")
		return z, o, err
	}
	o, err := u.manager.ArithmeticRealNumeralText("1")
	return z, o, err
}

// Decompose returns p[0]+x*p[1]+... and rejects non-arithmetic occurrences.
func (u *NonlinearArithmeticUtil) Decompose(variable *ast.ContainsApp, expression ast.ASTHandle) (*NLPolynomial, bool, error) {
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
	contains, err := variable.Check(expression)
	if err != nil {
		return nil, false, err
	}
	if !contains {
		p, err := newNLPolynomial(u.manager, []ast.ASTHandle{expression})
		return p, err == nil, err
	}
	if expression == target {
		z, o, err := u.zeroOne(target)
		if err != nil {
			return nil, false, err
		}
		p, err := newNLPolynomial(u.manager, []ast.ASTHandle{z, o})
		return p, err == nil, err
	}
	family, err := u.manager.GetFamilyID(expression)
	if err != nil || family != u.family {
		return nil, false, err
	}
	kind, err := u.manager.GetDeclKind(expression)
	if err != nil {
		return nil, false, err
	}
	n, err := u.manager.GetNumArgs(expression)
	if err != nil {
		return nil, false, err
	}
	if kind != 6 && kind != 7 && kind != 8 && kind != 9 {
		return nil, false, nil
	}
	if n == 0 {
		return nil, false, nil
	}
	first, err := u.manager.GetArg(expression, 0)
	if err != nil {
		return nil, false, err
	}
	result, ok, err := u.Decompose(variable, first)
	if err != nil || !ok {
		return result, ok, err
	}
	if kind == 8 {
		return result, true, u.Negate(result)
	}
	for i := uint32(1); i < n; i++ {
		arg, cause := u.manager.GetArg(expression, i)
		if cause != nil {
			_ = result.Close()
			return nil, false, cause
		}
		other, matched, cause := u.Decompose(variable, arg)
		if cause != nil || !matched {
			_ = result.Close()
			if other != nil {
				_ = other.Close()
			}
			return nil, matched, cause
		}
		if kind == 6 {
			cause = u.Add(result, other)
		} else if kind == 7 {
			cause = u.Subtract(result, other)
		} else {
			cause = u.Multiply(result, other)
		}
		cause2 := other.Close()
		if cause == nil {
			cause = cause2
		}
		if cause != nil {
			_ = result.Close()
			return nil, false, cause
		}
	}
	return result, true, nil
}
