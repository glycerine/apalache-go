package arith

// Source: src/tactic/arith/lia2card_tactic.cpp, get_pb_sum/get_sum and helpers.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// PBSum represents constant + sum(coefficients[i] * indicator(args[i])).
// All coefficients are positive. It owns the constructed Boolean expressions;
// callers must Close it before closing the manager.
type PBSum struct {
	Args         []ast.ASTHandle
	Coefficients []big.Rat
	Constant     big.Rat
	manager      *ast.ASTManager
	refs         []ast.ASTRef
}

func (s *PBSum) Close() error {
	if s == nil {
		return nil
	}
	for len(s.refs) > 0 {
		i := len(s.refs) - 1
		if err := s.manager.DelRef(&s.refs[i]); err != nil {
			return err
		}
		s.refs = s.refs[:i]
	}
	s.Args = nil
	s.Coefficients = nil
	return nil
}

func (s *PBSum) pin(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := s.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.refs = append(s.refs, ref)
	return h, nil
}

// lia2cardNumeral recognizes the source's recursive minus and to_real numeral
// wrappers, without accepting arbitrary arithmetic expressions as numerals.
func lia2cardNumeral(m *ast.ASTManager, h ast.ASTHandle, family ast.FamilyID) (*big.Rat, bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return nil, false, err
	}
	fid, err := m.GetFamilyID(h)
	if err != nil || fid != family {
		return nil, false, err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return nil, false, err
	}
	if op == 8 || op == 14 {
		arg, err := m.GetArg(h, 0)
		if err != nil {
			return nil, false, err
		}
		r, found, err := lia2cardNumeral(m, arg, family)
		if found && op == 8 {
			r.Neg(r)
		}
		return r, found, err
	}
	text, _, found, err := m.ArithmeticNumeralText(h)
	if err != nil || !found {
		return nil, false, err
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return r, true, nil
}

// ExtractPBSum borrows the arithmetic term and known 0/1 variables. It
// preserves source traversal order and duplicate weighted conditions. Failed
// extraction releases all temporary AST owners before returning.
func ExtractPBSum(m *ast.ASTManager, term ast.ASTHandle, zeroOne map[ast.ASTHandle]bool) (*PBSum, bool, error) {
	return extractPBSumWithTrace(m, term, zeroOne, nil)
}

func extractPBSumWithTrace(m *ast.ASTManager, term ast.ASTHandle, zeroOne map[ast.ASTHandle]bool, tags *diagnostic.TraceTags) (*PBSum, bool, error) {
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, false, err
	}
	s := &PBSum{manager: m}
	var conds []ast.ASTHandle
	insert := func(weight *big.Rat, x ast.ASTHandle) error {
		if !m.IsTrue(x) {
			conds = append(conds, x)
			defer func() { conds = conds[:len(conds)-1] }()
		}
		condition, err := s.pin(m.MkAndArray(conds))
		if err != nil {
			return err
		}
		if m.IsTrue(condition) {
			s.Constant.Add(&s.Constant, weight)
			return nil
		}
		if weight.Sign() == 0 {
			return nil
		}
		var coefficient big.Rat
		coefficient.Set(weight)
		if coefficient.Sign() < 0 {
			condition, err = s.pin(m.MkNot(condition))
			if err != nil {
				return err
			}
			coefficient.Neg(&coefficient)
			s.Constant.Add(&s.Constant, weight)
		}
		s.Args = append(s.Args, condition)
		s.Coefficients = append(s.Coefficients, coefficient)
		return nil
	}
	var getSum func(ast.ASTHandle, *big.Rat) (bool, error)
	getSum = func(h ast.ASTHandle, mul *big.Rat) (bool, error) {
		kind, err := m.GetKind(h)
		if err != nil || kind != ast.ASTApp {
			return false, err
		}
		fid, err := m.GetFamilyID(h)
		if err != nil {
			return false, err
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return false, err
		}
		if fid == family {
			n, err := m.GetNumArgs(h)
			if err != nil {
				return false, err
			}
			if op == 6 {
				for i := uint32(0); i < n; i++ {
					a, err := m.GetArg(h, i)
					if err != nil {
						return false, err
					}
					ok, err := getSum(a, mul)
					if err != nil || !ok {
						return ok, err
					}
				}
				return true, nil
			}
			if (op == 7 || op == 9) && n == 2 {
				left, err := m.GetArg(h, 0)
				if err != nil {
					return false, err
				}
				right, err := m.GetArg(h, 1)
				if err != nil {
					return false, err
				}
				if op == 7 {
					ok, err := getSum(left, mul)
					if err != nil || !ok {
						return ok, err
					}
					return getSum(right, new(big.Rat).Neg(mul))
				}
				for side := 0; side < 2; side++ {
					r, ok, err := lia2cardNumeral(m, left, family)
					if err != nil {
						return false, err
					}
					if ok {
						return getSum(right, new(big.Rat).Mul(r, mul))
					}
					left, right = right, left
				}
			} else if (op == 8 || op == 14) && n == 1 {
				arg, err := m.GetArg(h, 0)
				if err != nil {
					return false, err
				}
				if op == 8 {
					return getSum(arg, new(big.Rat).Neg(mul))
				}
				return getSum(arg, mul)
			}
		}
		ite, err := arithmeticBasicApp(m, h, ast.ASTOpIte)
		if err != nil {
			return false, err
		}
		if ite {
			condition, err := m.GetArg(h, 0)
			if err != nil {
				return false, err
			}
			left, err := m.GetArg(h, 1)
			if err != nil {
				return false, err
			}
			right, err := m.GetArg(h, 2)
			if err != nil {
				return false, err
			}
			conds = append(conds, condition)
			ok, err := getSum(left, mul)
			conds = conds[:len(conds)-1]
			if err != nil {
				return false, err
			}
			neg, err := s.pin(m.MkNot(condition))
			if err != nil {
				return false, err
			}
			conds = append(conds, neg)
			other, err := getSum(right, mul)
			conds = conds[:len(conds)-1]
			return ok && other, err
		}
		if zeroOne[h] {
			sort, err := m.GetSort(h)
			if err != nil {
				return false, err
			}
			k, err := m.GetDeclKind(sort)
			if err != nil {
				return false, err
			}
			var one ast.ASTHandle
			if k == 1 {
				one, err = s.pin(m.ArithmeticIntegerNumeralText("1"))
			} else {
				one, err = s.pin(m.ArithmeticRealNumeralText("1"))
			}
			if err != nil {
				return false, err
			}
			indicator, err := s.pin(m.MkEq(h, one))
			if err != nil {
				return false, err
			}
			return true, insert(mul, indicator)
		}
		r, ok, err := lia2cardNumeral(m, h, family)
		if err != nil || !ok {
			if err == nil {
				err = lia2cardTraceUnsupported(m, tags, h)
			}
			return ok, err
		}
		truth, err := m.MkTrue()
		if err != nil {
			return false, err
		}
		return true, insert(new(big.Rat).Mul(mul, r), truth)
	}
	ok, err := getSum(term, big.NewRat(1, 1))
	if err != nil || !ok {
		return nil, ok, errors.Join(err, s.Close())
	}
	return s, true, nil
}
