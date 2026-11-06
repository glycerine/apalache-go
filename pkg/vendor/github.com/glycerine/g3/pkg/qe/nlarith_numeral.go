package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

// Numeral evaluates the retained arithmetic numeral grammar (+,-,*,unary -).
func (u *NonlinearArithmeticUtil) Numeral(expression ast.ASTHandle) (*big.Rat, bool, error) {
	if err := u.open(); err != nil {
		return nil, false, err
	}
	if v, ok, err := u.term.numeral(expression); err != nil || ok {
		return v, ok, err
	}
	family, err := u.manager.GetFamilyID(expression)
	if err != nil || family != u.family {
		return nil, false, err
	}
	kind, err := u.manager.GetDeclKind(expression)
	if err != nil {
		return nil, false, err
	}
	if kind != 6 && kind != 7 && kind != 8 && kind != 9 {
		return nil, false, nil
	}
	n, err := u.manager.GetNumArgs(expression)
	if err != nil || n == 0 {
		return nil, false, err
	}
	a, err := u.manager.GetArg(expression, 0)
	if err != nil {
		return nil, false, err
	}
	result, ok, err := u.Numeral(a)
	if err != nil || !ok {
		return nil, ok, err
	}
	if kind == 8 {
		return new(big.Rat).Neg(result), true, nil
	}
	for i := uint32(1); i < n; i++ {
		arg, cause := u.manager.GetArg(expression, i)
		if cause != nil {
			return nil, false, cause
		}
		v, matched, cause := u.Numeral(arg)
		if cause != nil || !matched {
			return nil, matched, cause
		}
		switch kind {
		case 6:
			result.Add(result, v)
		case 7:
			result.Sub(result, v)
		case 9:
			result.Mul(result, v)
		}
	}
	return result, true, nil
}
