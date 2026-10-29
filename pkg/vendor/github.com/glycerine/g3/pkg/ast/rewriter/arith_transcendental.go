package rewriter

// Source: src/ast/rewriter/arith_rewriter.cpp:1154-1655.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

const (
	arithOpSin   int32 = 19
	arithOpCos   int32 = 20
	arithOpTan   int32 = 21
	arithOpASin  int32 = 22
	arithOpACos  int32 = 23
	arithOpATan  int32 = 24
	arithOpSinh  int32 = 25
	arithOpCosh  int32 = 26
	arithOpTanh  int32 = 27
	arithOpASinh int32 = 28
	arithOpACosh int32 = 29
	arithOpATanh int32 = 30
	arithOpPi    int32 = 31
)

func (r *ArithRewriter) pi() (ast.ASTHandle, error) { return r.app(arithOpPi) }

func (r *ArithRewriter) negativeTerm(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	_, args, mul, err := r.operation(h, arithOpMul)
	if err != nil || !mul || len(args) != 2 {
		return ast.ASTHandle{}, false, err
	}
	coefficient, _, numeral, err := r.numeral(args[0])
	if err != nil || !numeral || coefficient.Cmp(big.NewRat(-1, 1)) != 0 {
		return ast.ASTHandle{}, false, err
	}
	return args[1], true, nil
}

func (r *ArithRewriter) piMultiple(h ast.ASTHandle) (*big.Rat, bool, error) {
	_, args, pi, err := r.operation(h, arithOpPi)
	if err != nil {
		return nil, false, err
	}
	if pi && len(args) == 0 {
		return big.NewRat(1, 1), true, nil
	}
	_, args, mul, err := r.operation(h, arithOpMul)
	if err != nil || !mul || len(args) != 2 {
		return nil, false, err
	}
	_, piArgs, pi, err := r.operation(args[1], arithOpPi)
	if err != nil || !pi || len(piArgs) != 0 {
		return nil, false, err
	}
	coefficient, _, numeral, err := r.numeral(args[0])
	return coefficient, numeral, err
}

func arithPiPeriod(k *big.Rat) *big.Rat {
	floor := arithFloor(k)
	whole := new(big.Int).Mod(new(big.Int).Set(floor), big.NewInt(2))
	fraction := new(big.Rat).Sub(k, new(big.Rat).SetInt(floor))
	return fraction.Add(fraction, new(big.Rat).SetInt(whole))
}

func (r *ArithRewriter) sqrt(value *big.Rat) (ast.ASTHandle, error) {
	base, err := r.makeNumeral(value, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	half, err := r.makeNumeral(big.NewRat(1, 2), false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.app(arithOpPower, base, half)
}

func (r *ArithRewriter) signed(h ast.ASTHandle, negative bool) (ast.ASTHandle, error) {
	if !negative {
		return h, nil
	}
	return r.app(arithOpUMinus, h)
}

func (r *ArithRewriter) sinValue(k *big.Rat) (ast.ASTHandle, bool, error) {
	k = arithPiPeriod(k)
	negative := k.Cmp(big.NewRat(1, 1)) >= 0
	if negative {
		k.Sub(k, big.NewRat(1, 1))
	}
	makeRat := func(v *big.Rat) (ast.ASTHandle, bool, error) {
		if negative {
			v.Neg(v)
		}
		h, err := r.makeNumeral(v, false)
		return h, err == nil, err
	}
	if k.Sign() == 0 || k.Cmp(big.NewRat(1, 1)) == 0 {
		return makeRat(new(big.Rat))
	}
	if k.Cmp(big.NewRat(1, 2)) == 0 {
		return makeRat(big.NewRat(1, 1))
	}
	if k.Cmp(big.NewRat(1, 6)) == 0 || k.Cmp(big.NewRat(5, 6)) == 0 {
		return makeRat(big.NewRat(1, 2))
	}
	if k.Cmp(big.NewRat(1, 4)) == 0 || k.Cmp(big.NewRat(3, 4)) == 0 {
		h, err := r.sqrt(big.NewRat(1, 2))
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err = r.signed(h, negative)
		return h, err == nil, err
	}
	if k.Cmp(big.NewRat(1, 3)) == 0 || k.Cmp(big.NewRat(2, 3)) == 0 {
		root, err := r.sqrt(big.NewRat(3, 1))
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		two, err := r.makeNumeral(big.NewRat(2, 1), false)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err := r.app(arithOpDiv, root, two)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err = r.signed(h, negative)
		return h, err == nil, err
	}
	if k.Cmp(big.NewRat(1, 12)) == 0 || k.Cmp(big.NewRat(11, 12)) == 0 || k.Cmp(big.NewRat(5, 12)) == 0 || k.Cmp(big.NewRat(7, 12)) == 0 {
		six, err := r.sqrt(big.NewRat(6, 1))
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		twoRoot, err := r.sqrt(big.NewRat(2, 1))
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		operator := arithOpSub
		if k.Cmp(big.NewRat(5, 12)) == 0 || k.Cmp(big.NewRat(7, 12)) == 0 {
			operator = arithOpAdd
		}
		numerator, err := r.app(operator, six, twoRoot)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		four, err := r.makeNumeral(big.NewRat(4, 1), false)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err := r.app(arithOpDiv, numerator, four)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err = r.signed(h, negative)
		return h, err == nil, err
	}
	return ast.ASTHandle{}, false, nil
}

func (r *ArithRewriter) piOffset(h ast.ASTHandle) (*big.Rat, ast.ASTHandle, bool, error) {
	_, args, add, err := r.operation(h, arithOpAdd)
	if err != nil || !add {
		return nil, ast.ASTHandle{}, false, err
	}
	for _, arg := range args {
		coefficient, yes, e := r.piMultiple(arg)
		if e != nil {
			return nil, ast.ASTHandle{}, false, e
		}
		if yes {
			return coefficient, arg, true, nil
		}
	}
	return nil, ast.ASTHandle{}, false, nil
}

func (r *ArithRewriter) integerPiMultiple(h ast.ASTHandle, even bool) (bool, error) {
	_, args, mul, err := r.operation(h, arithOpMul)
	if err != nil || !mul || len(args) != 2 {
		return false, err
	}
	isPair := func(left, right ast.ASTHandle) (bool, error) {
		_, piArgs, pi, e := r.operation(left, arithOpPi)
		if e != nil {
			return false, e
		}
		toReal, _, e := r.unary(right, arithOpToReal)
		return pi && len(piArgs) == 0 && toReal, e
	}
	coefficient := big.NewRat(1, 1)
	pair := args
	if value, _, numeral, e := r.numeral(args[0]); e != nil {
		return false, e
	} else if numeral {
		if !value.IsInt() {
			return false, nil
		}
		coefficient = value
		_, pair, mul, err = r.operation(args[1], arithOpMul)
		if err != nil || !mul || len(pair) != 2 {
			return false, err
		}
	}
	if even && new(big.Int).Mod(coefficient.Num(), big.NewInt(2)).Sign() != 0 {
		return false, nil
	}
	if yes, e := isPair(pair[0], pair[1]); e != nil || yes {
		return yes, e
	}
	return isPair(pair[1], pair[0])
}

func (r *ArithRewriter) integerPiOffset(h ast.ASTHandle, even bool) (ast.ASTHandle, bool, error) {
	_, args, add, err := r.operation(h, arithOpAdd)
	if err != nil || !add {
		return ast.ASTHandle{}, false, err
	}
	for _, arg := range args {
		yes, e := r.integerPiMultiple(arg, even)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if yes {
			return arg, true, nil
		}
	}
	return ast.ASTHandle{}, false, nil
}

func (r *ArithRewriter) subtractOffset(arg, offset ast.ASTHandle) (ast.ASTHandle, error) {
	return r.app(arithOpSub, arg, offset)
}

func (r *ArithRewriter) rewriteSin(arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if yes, child, err := r.unary(arg, arithOpASin); err != nil {
		return RewriteFailed, err
	} else if yes {
		return RewriteDone, r.manager.SetRef(out, child)
	}
	if yes, child, err := r.unary(arg, arithOpACos); err != nil {
		return RewriteFailed, err
	} else if yes {
		one, e := r.makeNumeral(big.NewRat(1, 1), false)
		if e != nil {
			return RewriteFailed, e
		}
		square, e := r.app(arithOpMul, child, child)
		if e != nil {
			return RewriteFailed, e
		}
		difference, e := r.app(arithOpSub, one, square)
		if e != nil {
			return RewriteFailed, e
		}
		root, e := r.sqrt(big.NewRat(1, 1))
		if e != nil {
			return RewriteFailed, e
		}
		half, e := r.manager.GetArg(root, 1)
		if e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpPower, difference, half)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteFull, r.manager.SetRef(out, result)
	}
	if value, _, numeral, err := r.numeral(arg); err != nil {
		return RewriteFailed, err
	} else if numeral && value.Sign() == 0 {
		return RewriteDone, r.manager.SetRef(out, arg)
	}
	if coefficient, yes, err := r.piMultiple(arg); err != nil {
		return RewriteFailed, err
	} else if yes {
		result, matched, e := r.sinValue(coefficient)
		if e != nil {
			return RewriteFailed, e
		}
		if matched {
			return RewriteFull, r.manager.SetRef(out, result)
		}
	}
	if coefficient, offset, yes, err := r.piOffset(arg); err != nil {
		return RewriteFailed, err
	} else if yes {
		period := arithPiPeriod(coefficient)
		base, e := r.subtractOffset(arg, offset)
		if e != nil {
			return RewriteFailed, e
		}
		var result ast.ASTHandle
		status := Rewrite2
		switch {
		case period.Sign() == 0:
			result, e = r.app(arithOpSin, base)
		case period.Cmp(big.NewRat(1, 2)) == 0:
			result, e = r.app(arithOpCos, base)
		case period.Cmp(big.NewRat(1, 1)) == 0:
			result, e = r.app(arithOpSin, base)
			if e == nil {
				result, e = r.app(arithOpUMinus, result)
			}
			status = Rewrite3
		case period.Cmp(big.NewRat(3, 2)) == 0:
			result, e = r.app(arithOpCos, base)
			if e == nil {
				result, e = r.app(arithOpUMinus, result)
			}
			status = Rewrite3
		default:
			return RewriteFailed, nil
		}
		if e != nil {
			return RewriteFailed, e
		}
		return status, r.manager.SetRef(out, result)
	}
	if offset, yes, err := r.integerPiOffset(arg, true); err != nil {
		return RewriteFailed, err
	} else if yes {
		base, e := r.subtractOffset(arg, offset)
		if e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpSin, base)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	return RewriteFailed, nil
}

func (r *ArithRewriter) rewriteCos(arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if yes, child, err := r.unary(arg, arithOpACos); err != nil {
		return RewriteFailed, err
	} else if yes {
		return RewriteDone, r.manager.SetRef(out, child)
	}
	if value, _, numeral, err := r.numeral(arg); err != nil {
		return RewriteFailed, err
	} else if numeral && value.Sign() == 0 {
		one, e := r.makeNumeral(big.NewRat(1, 1), false)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, one)
	}
	if coefficient, yes, err := r.piMultiple(arg); err != nil {
		return RewriteFailed, err
	} else if yes {
		coefficient.Add(coefficient, big.NewRat(1, 2))
		result, matched, e := r.sinValue(coefficient)
		if e != nil {
			return RewriteFailed, e
		}
		if matched {
			return RewriteFull, r.manager.SetRef(out, result)
		}
	}
	if coefficient, offset, yes, err := r.piOffset(arg); err != nil {
		return RewriteFailed, err
	} else if yes {
		period := arithPiPeriod(coefficient)
		base, e := r.subtractOffset(arg, offset)
		if e != nil {
			return RewriteFailed, e
		}
		var result ast.ASTHandle
		status := Rewrite2
		switch {
		case period.Sign() == 0:
			result, e = r.app(arithOpCos, base)
		case period.Cmp(big.NewRat(1, 2)) == 0:
			result, e = r.app(arithOpSin, base)
			if e == nil {
				result, e = r.app(arithOpUMinus, result)
			}
			status = Rewrite3
		case period.Cmp(big.NewRat(1, 1)) == 0:
			result, e = r.app(arithOpCos, base)
			if e == nil {
				result, e = r.app(arithOpUMinus, result)
			}
			status = Rewrite3
		case period.Cmp(big.NewRat(3, 2)) == 0:
			result, e = r.app(arithOpSin, base)
		default:
			return RewriteFailed, nil
		}
		if e != nil {
			return RewriteFailed, e
		}
		return status, r.manager.SetRef(out, result)
	}
	if offset, yes, err := r.integerPiOffset(arg, true); err != nil {
		return RewriteFailed, err
	} else if yes {
		base, e := r.subtractOffset(arg, offset)
		if e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpCos, base)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	return RewriteFailed, nil
}

func (r *ArithRewriter) rewriteTan(arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if yes, child, err := r.unary(arg, arithOpATan); err != nil {
		return RewriteFailed, err
	} else if yes {
		return RewriteDone, r.manager.SetRef(out, child)
	}
	if value, _, numeral, err := r.numeral(arg); err != nil {
		return RewriteFailed, err
	} else if numeral && value.Sign() == 0 {
		return RewriteDone, r.manager.SetRef(out, arg)
	}
	if coefficient, yes, err := r.piMultiple(arg); err != nil {
		return RewriteFailed, err
	} else if yes {
		numerator, matched, e := r.sinValue(coefficient)
		if e != nil {
			return RewriteFailed, e
		}
		if matched {
			if zero, e := r.zero(numerator); e != nil {
				return RewriteFailed, e
			} else if zero {
				return RewriteDone, r.manager.SetRef(out, numerator)
			}
			denominator, denominatorMatched, e := r.sinValue(new(big.Rat).Add(coefficient, big.NewRat(1, 2)))
			if e != nil {
				return RewriteFailed, e
			}
			if denominatorMatched {
				if zero, e := r.zero(denominator); e != nil {
					return RewriteFailed, e
				} else if !zero {
					result, e := r.app(arithOpDiv, numerator, denominator)
					if e != nil {
						return RewriteFailed, e
					}
					return RewriteFull, r.manager.SetRef(out, result)
				}
			}
		}
	}
	if coefficient, offset, yes, err := r.piOffset(arg); err != nil {
		return RewriteFailed, err
	} else if yes {
		fraction := new(big.Rat).Sub(coefficient, new(big.Rat).SetInt(arithFloor(coefficient)))
		if fraction.Sign() == 0 {
			base, e := r.subtractOffset(arg, offset)
			if e != nil {
				return RewriteFailed, e
			}
			result, e := r.app(arithOpTan, base)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, result)
		}
	}
	if offset, yes, err := r.integerPiOffset(arg, false); err != nil {
		return RewriteFailed, err
	} else if yes {
		base, e := r.subtractOffset(arg, offset)
		if e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpTan, base)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	if r.expandTan {
		sine, err := r.app(arithOpSin, arg)
		if err != nil {
			return RewriteFailed, err
		}
		cosine, err := r.app(arithOpCos, arg)
		if err != nil {
			return RewriteFailed, err
		}
		result, err := r.app(arithOpDiv, sine, cosine)
		if err != nil {
			return RewriteFailed, err
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	return RewriteFailed, nil
}

func (r *ArithRewriter) piFraction(value *big.Rat) (ast.ASTHandle, error) {
	coefficient, err := r.makeNumeral(value, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pi, err := r.pi()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.app(arithOpMul, coefficient, pi)
}

func (r *ArithRewriter) rewriteInverseTrig(kind int32, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	value, _, numeral, err := r.numeral(arg)
	if err != nil {
		return RewriteFailed, err
	}
	if numeral {
		if value.Sign() == 0 {
			if kind == arithOpACos {
				h, e := r.piFraction(big.NewRat(1, 2))
				if e != nil {
					return RewriteFailed, e
				}
				return Rewrite2, r.manager.SetRef(out, h)
			}
			return RewriteDone, r.manager.SetRef(out, arg)
		}
		if (kind == arithOpASin || kind == arithOpATan) && value.Cmp(big.NewRat(-1, 1)) < 0 {
			positive, e := r.makeNumeral(new(big.Rat).Neg(value), false)
			if e != nil {
				return RewriteFailed, e
			}
			inner, e := r.app(kind, positive)
			if e != nil {
				return RewriteFailed, e
			}
			result, e := r.app(arithOpUMinus, inner)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, result)
		}
		var coefficient *big.Rat
		switch kind {
		case arithOpASin:
			if value.Cmp(big.NewRat(1, 1)) == 0 {
				coefficient = big.NewRat(1, 2)
			} else if value.Cmp(big.NewRat(-1, 1)) == 0 {
				coefficient = big.NewRat(-1, 2)
			} else if value.Cmp(big.NewRat(1, 2)) == 0 {
				coefficient = big.NewRat(1, 6)
			} else if value.Cmp(big.NewRat(-1, 2)) == 0 {
				coefficient = big.NewRat(-1, 6)
			}
		case arithOpACos:
			if value.Cmp(big.NewRat(1, 1)) == 0 {
				zero, e := r.makeNumeral(new(big.Rat), false)
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, r.manager.SetRef(out, zero)
			} else if value.Cmp(big.NewRat(-1, 1)) == 0 {
				pi, e := r.pi()
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, r.manager.SetRef(out, pi)
			} else if value.Cmp(big.NewRat(1, 2)) == 0 {
				coefficient = big.NewRat(1, 3)
			} else if value.Cmp(big.NewRat(-1, 2)) == 0 {
				coefficient = big.NewRat(2, 3)
			}
		case arithOpATan:
			if value.Cmp(big.NewRat(1, 1)) == 0 {
				coefficient = big.NewRat(1, 4)
			} else if value.Cmp(big.NewRat(-1, 1)) == 0 {
				coefficient = big.NewRat(-1, 4)
			}
		}
		if coefficient != nil {
			h, e := r.piFraction(coefficient)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, h)
		}
	}
	if kind == arithOpASin || kind == arithOpATan {
		if child, yes, e := r.negativeTerm(arg); e != nil {
			return RewriteFailed, e
		} else if yes {
			inner, e := r.app(kind, child)
			if e != nil {
				return RewriteFailed, e
			}
			negative, e := r.app(arithOpUMinus, inner)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, negative)
		}
	}
	return RewriteFailed, nil
}

func (r *ArithRewriter) rewriteHyperbolic(kind int32, inverse int32, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if yes, child, err := r.unary(arg, inverse); err != nil {
		return RewriteFailed, err
	} else if yes {
		return RewriteDone, r.manager.SetRef(out, child)
	}
	child, negative, err := r.negativeTerm(arg)
	if err != nil || !negative {
		return RewriteFailed, err
	}
	inner, err := r.app(kind, child)
	if err != nil {
		return RewriteFailed, err
	}
	if kind == arithOpCosh {
		return RewriteDone, r.manager.SetRef(out, inner)
	}
	result, err := r.app(arithOpUMinus, inner)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, r.manager.SetRef(out, result)
}
