package rewriter

// Source: src/ast/rewriter/bit2int.cpp:185-235.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (b *Bit2Int) isArithmeticOperation(term ast.ASTHandle, kind int32) (bool, error) {
	actualKind, err := b.manager.GetKind(term)
	if err != nil || actualKind != ast.ASTApp {
		return false, err
	}
	family, err := b.manager.GetFamilyID(term)
	if err != nil || family != b.arithFamily {
		return false, err
	}
	op, err := b.manager.GetDeclKind(term)
	return op == kind, err
}

func (b *Bit2Int) arithmeticInteger(term ast.ASTHandle) (*big.Int, bool, error) {
	text, integer, matched, err := b.manager.ArithmeticNumeralText(term)
	if err != nil || !matched || !integer {
		return nil, false, err
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return value, true, nil
}

func (b *Bit2Int) positiveArithmeticNumeral(c *bit2IntCall, value *big.Int) (ast.ASTHandle, error) {
	service := b.manager.NumeralService()
	rational, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer service.Del(&rational)
	if err = service.SetString(&rational, value.String()); err != nil {
		return ast.ASTHandle{}, err
	}
	term, err := b.manager.ArithmeticNumeral(&rational, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(term)
}

func (b *Bit2Int) isBVPoly(c *bit2IntCall, root ast.ASTHandle) (pos, neg ast.ASTHandle, matched bool, err error) {
	zero, err := b.manager.RefGet(b.bit0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	pos, err = b.mkBV2Int(c, zero)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	neg = pos
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		term := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		bv2int, err := b.manager.IsBitVectorOperation(term, 58)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if bv2int {
			pos, matched, err = b.mkAdd(c, term, pos)
			if err != nil || !matched {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			continue
		}
		value, numeral, err := b.arithmeticInteger(term)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if numeral {
			if value.Sign() >= 0 {
				pos, matched, err = b.mkAdd(c, term, pos)
			} else {
				positive, x := b.positiveArithmeticNumeral(c, new(big.Int).Neg(value))
				if x != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, x
				}
				neg, matched, err = b.mkAdd(c, positive, neg)
			}
			if err != nil || !matched {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			continue
		}
		add, err := b.isArithmeticOperation(term, 6)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if add {
			count, err := b.manager.GetNumArgs(term)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			for i := uint32(0); i < count; i++ {
				arg, err := b.manager.GetArg(term, i)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				stack = append(stack, arg)
			}
			continue
		}
		mul, err := b.isArithmeticOperation(term, 9)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if mul {
			count, err := b.manager.GetNumArgs(term)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			if count == 2 {
				negativeProduct := false
				left, err := b.manager.GetArg(term, 0)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				right, err := b.manager.GetArg(term, 1)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
					coefficient, numeral, err := b.arithmeticInteger(pair[0])
					if err != nil {
						return ast.ASTHandle{}, ast.ASTHandle{}, false, err
					}
					other, err := b.manager.IsBitVectorOperation(pair[1], 58)
					if err != nil {
						return ast.ASTHandle{}, ast.ASTHandle{}, false, err
					}
					if numeral && coefficient.Cmp(big.NewInt(-1)) == 0 && other {
						neg, matched, err = b.mkAdd(c, pair[1], neg)
						if err != nil || !matched {
							return ast.ASTHandle{}, ast.ASTHandle{}, false, err
						}
						negativeProduct = true
						break
					}
				}
				if negativeProduct {
					continue
				}
			}
		}
		unaryMinus, err := b.isArithmeticOperation(term, 8)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if unaryMinus {
			count, err := b.manager.GetNumArgs(term)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			if count == 1 {
				arg, err := b.manager.GetArg(term, 0)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				bv2int, err := b.manager.IsBitVectorOperation(arg, 58)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				if bv2int {
					neg, matched, err = b.mkAdd(c, arg, neg)
					if err != nil || !matched {
						return ast.ASTHandle{}, ast.ASTHandle{}, false, err
					}
					continue
				}
			}
		}
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	return pos, neg, true, nil
}
