package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"strconv"
)

// NLRootForm represents (A + Multiplier*sqrt(C))/D.
type NLRootForm struct {
	A          ast.ASTHandle
	Multiplier int
	C, D       ast.ASTHandle
}

func (u *NonlinearArithmeticUtil) RootExpression(root NLRootForm) (ast.ASTHandle, error) {
	zeroC, err := u.definitelyZero(root.C)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if zeroC || root.Multiplier == 0 {
		return u.manager.MkArithmeticDiv(root.A, root.D)
	}
	z, err := u.term.zero(root.C)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negative, err := u.manager.MkArithmeticLT(root.C, z)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	minus, err := u.manager.MkArithmeticNeg(root.C)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	absolute, err := u.manager.MkIte(negative, minus, root.C)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	half, err := u.realNumeral("1/2")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sqrt, err := u.manager.MkArithmeticPower(absolute, half)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	multiplier, err := u.realNumeral(strconv.Itoa(root.Multiplier))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	scaled, err := u.manager.MkArithmeticMul(multiplier, sqrt)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	numerator, err := u.manager.MkArithmeticAdd(root.A, scaled)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return u.manager.MkArithmeticDiv(numerator, root.D)
}
