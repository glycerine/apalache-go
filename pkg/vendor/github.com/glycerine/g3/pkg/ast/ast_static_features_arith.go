package ast

// Source: src/ast/static_features.cpp:103-152,211-216,246-272,282-296,313-325,346-361;
// src/ast/static_features.h:116-127,132-141.

func (f *StaticFeatures) isArithmeticExpr(h ASTHandle) (bool, error) {
	kind, err := f.manager.GetKind(h)
	if err != nil || kind != ASTApp {
		return false, err
	}
	fid, err := f.manager.GetFamilyID(h)
	return fid == f.arithFID, err
}

func (f *StaticFeatures) arithmeticOp(h ASTHandle, op int32) (bool, error) {
	yes, err := f.isArithmeticExpr(h)
	if err != nil || !yes {
		return false, err
	}
	kind, err := f.manager.GetDeclKind(h)
	return kind == op, err
}

func (f *StaticFeatures) readArithmeticNumeral(h ASTHandle) (bool, error) {
	text, _, matched, err := f.manager.ArithmeticNumeralText(h)
	if err != nil || !matched {
		return matched, err
	}
	return true, f.manager.numerals.SetString(&f.arithScratch, text)
}

func (f *StaticFeatures) accScratch() error {
	negative, err := f.manager.numerals.IsNeg(&f.arithScratch)
	if err != nil {
		return err
	}
	if negative {
		return f.manager.numerals.SubAssign(&f.ArithKSum, &f.arithScratch)
	}
	return f.manager.numerals.AddAssign(&f.ArithKSum, &f.arithScratch)
}

func (f *StaticFeatures) accNumeral(h ASTHandle) error {
	yes, err := f.readArithmeticNumeral(h)
	if err != nil || !yes {
		return err
	}
	return f.accScratch()
}

// isDiffTerm leaves its source coefficient in arithScratch when it succeeds.
func (f *StaticFeatures) isDiffTerm(h ASTHandle) (bool, error) {
	arith, err := f.isArithmeticExpr(h)
	if err != nil {
		return false, err
	}
	if !arith {
		if err = f.manager.numerals.Reset(&f.arithScratch); err != nil {
			return false, err
		}
		return true, nil
	}
	if yes, e := f.readArithmeticNumeral(h); e != nil || yes {
		return yes, e
	}
	add, err := f.arithmeticOp(h, 6)
	if err != nil || !add {
		return false, err
	}
	n, err := f.manager.GetNumArgs(h)
	if err != nil || n != 2 {
		return false, err
	}
	coefficient, err := f.manager.GetArg(h, 0)
	if err != nil {
		return false, err
	}
	if yes, e := f.readArithmeticNumeral(coefficient); e != nil || !yes {
		return false, e
	}
	term, err := f.manager.GetArg(h, 1)
	if err != nil {
		return false, err
	}
	termArith, err := f.isArithmeticExpr(term)
	return !termArith, err
}

func (f *StaticFeatures) isMinusOne(h ASTHandle) (bool, error) {
	yes, err := f.readArithmeticNumeral(h)
	if err != nil || !yes {
		return false, err
	}
	return f.manager.numerals.IsMinusOne(&f.arithScratch)
}

func (f *StaticFeatures) isMinusOneProduct(h ASTHandle) (bool, error) {
	mul, err := f.arithmeticOp(h, 9)
	if err != nil || !mul {
		return false, err
	}
	n, err := f.manager.GetNumArgs(h)
	if err != nil || n != 2 {
		return false, err
	}
	coefficient, err := f.manager.GetArg(h, 0)
	if err != nil {
		return false, err
	}
	minusOne, err := f.isMinusOne(coefficient)
	if err != nil || !minusOne {
		return false, err
	}
	term, err := f.manager.GetArg(h, 1)
	if err != nil {
		return false, err
	}
	arith, err := f.isArithmeticExpr(term)
	return !arith, err
}

func (f *StaticFeatures) isDiffAtom(h ASTHandle, isEquality bool) (bool, error) {
	boolExpr, err := f.isBool(h)
	if err != nil || !boolExpr {
		return false, err
	}
	arith, err := f.isArithmeticExpr(h)
	if err != nil || !isEquality && !arith {
		return false, err
	}
	n, err := f.manager.GetNumArgs(h)
	if err != nil || n != 2 {
		return false, err
	}
	lhs, err := f.manager.GetArg(h, 0)
	if err != nil {
		return false, err
	}
	rhs, err := f.manager.GetArg(h, 1)
	if err != nil {
		return false, err
	}
	lhsArith, err := f.isArithmeticExpr(lhs)
	if err != nil {
		return false, err
	}
	rhsArith, err := f.isArithmeticExpr(rhs)
	if err != nil {
		return false, err
	}
	if !lhsArith && !rhsArith {
		return true, nil
	}
	if numeral, e := f.readArithmeticNumeral(rhs); e != nil || !numeral {
		return false, e
	}
	if !lhsArith {
		return true, nil
	}
	add, err := f.arithmeticOp(lhs, 6)
	if err != nil || !add {
		return false, err
	}
	if n, e := f.manager.GetNumArgs(lhs); e != nil || n != 2 {
		return false, e
	}
	first, err := f.manager.GetArg(lhs, 0)
	if err != nil {
		return false, err
	}
	second, err := f.manager.GetArg(lhs, 1)
	if err != nil {
		return false, err
	}
	firstArith, err := f.isArithmeticExpr(first)
	if err != nil {
		return false, err
	}
	if !firstArith {
		return f.isMinusOneProduct(second)
	}
	secondArith, err := f.isArithmeticExpr(second)
	if err != nil || secondArith {
		return false, err
	}
	return f.isMinusOneProduct(first)
}

func (f *StaticFeatures) accountArithmetic(h ASTHandle, isEquality bool) error {
	op, arith, err := int32(-1), false, error(nil)
	if arith, err = f.isArithmeticExpr(h); err != nil {
		return err
	}
	if arith {
		op, err = f.manager.GetDeclKind(h)
		if err != nil {
			return err
		}
	}
	if arith && (op == 2 || op == 3) {
		f.NumArithInequalities++
		if yes, e := f.isDiffAtom(h, false); e != nil {
			return e
		} else if yes {
			f.NumDiffInequalities++
		}
		lhs, e := f.manager.GetArg(h, 0)
		if e != nil {
			return e
		}
		if nested, e := f.isArithmeticExpr(lhs); e != nil {
			return e
		} else if !nested {
			f.NumSimpleInequalities++
		}
		rhs, e := f.manager.GetArg(h, 1)
		if e != nil {
			return e
		}
		if e = f.accNumeral(rhs); e != nil {
			return e
		}
	}
	if numeral, e := f.readArithmeticNumeral(h); e != nil {
		return e
	} else if numeral {
		integer, e := f.manager.numerals.IsInt(&f.arithScratch)
		if e != nil {
			return e
		}
		if !integer {
			f.HasRational = true
		}
	}
	if isEquality {
		lhs, e := f.manager.GetArg(h, 0)
		if e != nil {
			return e
		}
		rhs, e := f.manager.GetArg(h, 1)
		if e != nil {
			return e
		}
		if numeral, e := f.readArithmeticNumeral(rhs); e != nil {
			return e
		} else if numeral {
			if e = f.accScratch(); e != nil {
				return e
			}
			f.NumArithEqualities++
			if yes, e := f.isDiffAtom(h, true); e != nil {
				return e
			} else if yes {
				f.NumDiffEqualities++
			}
			if nested, e := f.isArithmeticExpr(lhs); e != nil {
				return e
			} else if !nested {
				f.NumSimpleEqualities++
			}
		}
	}
	if !arith {
		return nil
	}
	n, err := f.manager.GetNumArgs(h)
	if err != nil {
		return err
	}
	switch op {
	case 9:
		if n > 2 {
			f.NumNonLinear++
		} else if n > 0 {
			first, e := f.manager.GetArg(h, 0)
			if e != nil {
				return e
			}
			if numeral, e := f.readArithmeticNumeral(first); e != nil {
				return e
			} else if !numeral {
				f.NumNonLinear++
			}
		}
	case 10, 11, 12, 13:
		if n > 1 {
			divisor, e := f.manager.GetArg(h, 1)
			if e != nil {
				return e
			}
			if numeral, e := f.readArithmeticNumeral(divisor); e != nil {
				return e
			} else if !numeral {
				f.NumNonLinear++
			}
		}
	}
	return nil
}

func (f *StaticFeatures) accountTermITE(h ASTHandle) error {
	for i := uint32(1); i < 3; i++ {
		arg, err := f.manager.GetArg(h, i)
		if err != nil {
			return err
		}
		if err = f.accNumeral(arg); err != nil {
			return err
		}
		sort, err := f.manager.GetSort(arg)
		if err != nil {
			return err
		}
		fid, err := f.manager.GetFamilyID(sort)
		if err != nil {
			return err
		}
		if fid != f.arithFID {
			continue
		}
		f.NumArithTerms++
		if yes, e := f.isDiffTerm(arg); e != nil {
			return e
		} else if yes {
			f.NumDiffTerms++
			if e = f.accScratch(); e != nil {
				return e
			}
		}
	}
	return nil
}
