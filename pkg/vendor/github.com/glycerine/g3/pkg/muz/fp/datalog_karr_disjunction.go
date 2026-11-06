package fp

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// Source karr_relation::filter_interpreted accepts a binary disjunction of
// equalities to the same variable and appends its lower and upper bounds.
func (a *karrDatalogActivation) compileKarrDisjunction(guard ast.ASTHandle, positions map[ast.ASTHandle]uint32, arity uint32, out *karrGuardRule) error {
	parse := func(eq ast.ASTHandle) (ast.ASTHandle, *big.Rat, error) {
		var none ast.ASTHandle
		kind, err := a.manager.GetKind(eq)
		if err != nil {
			return none, nil, err
		}
		if kind != ast.ASTApp {
			return none, nil, ErrKarrDatalogFragment
		}
		family, err := a.manager.GetFamilyID(eq)
		if err != nil {
			return none, nil, err
		}
		op, err := a.manager.GetDeclKind(eq)
		if err != nil {
			return none, nil, err
		}
		n, err := a.manager.GetNumArgs(eq)
		if err != nil {
			return none, nil, err
		}
		if family != a.manager.BasicFamilyID() || op != ast.ASTOpEq || n != 2 {
			return none, nil, ErrKarrDatalogFragment
		}
		left, err := a.manager.GetArg(eq, 0)
		if err != nil {
			return none, nil, err
		}
		right, err := a.manager.GetArg(eq, 1)
		if err != nil {
			return none, nil, err
		}
		if _, ok := positions[left]; !ok {
			left, right = right, left
		}
		if _, ok := positions[left]; !ok {
			return none, nil, ErrKarrDatalogFragment
		}
		text, _, matched, err := a.manager.ArithmeticNumeralText(right)
		if err != nil {
			return none, nil, err
		}
		if !matched {
			return none, nil, ErrKarrDatalogFragment
		}
		value, ok := new(big.Rat).SetString(text)
		if !ok || !value.IsInt() {
			return none, nil, ErrKarrDatalogFragment
		}
		return left, value, nil
	}
	left, err := a.manager.GetArg(guard, 0)
	if err != nil {
		return err
	}
	right, err := a.manager.GetArg(guard, 1)
	if err != nil {
		return err
	}
	v, lo, err := parse(left)
	if err != nil {
		return err
	}
	w, hi, err := parse(right)
	if err != nil {
		return err
	}
	if v != w {
		return ErrKarrDatalogFragment
	}
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	out.row = make([]big.Rat, arity)
	out.row[positions[v]].SetInt64(1)
	out.constant.Neg(lo)
	out.secondRow = make([]big.Rat, arity)
	out.secondRow[positions[v]].SetInt64(-1)
	out.secondConstant.Set(hi)
	return nil
}
