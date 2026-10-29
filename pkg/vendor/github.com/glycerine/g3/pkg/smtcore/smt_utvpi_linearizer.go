package smtcore

// Source: src/smt/theory_utvpi.cpp utvpi_tester::linearize overloads.

import (
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type smtUTVPITerm struct {
	expression  ast.ASTHandle
	coefficient big.Rat
}

type smtUTVPILinearizer struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	terms   []smtUTVPITerm
	weight  big.Rat
}

func newSMTUTVPILinearizer(m *ast.ASTManager) (*smtUTVPILinearizer, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	return &smtUTVPILinearizer{manager: m, family: family}, nil
}

func (l *smtUTVPILinearizer) numeral(expression ast.ASTHandle) (*big.Rat, bool, error) {
	text, _, matched, err := l.manager.ArithmeticNumeralText(expression)
	if err != nil || !matched {
		return nil, false, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return value, true, nil
}

// A null right operand selects the source single-term overload. Successful
// output is left-right = sum(terms)+weight, with at most two unit coefficients.
func (l *smtUTVPILinearizer) linearize(left, right ast.ASTHandle) (bool, error) {
	l.terms = nil
	l.weight.SetInt64(0)
	work := []smtUTVPITerm{{left, *big.NewRat(1, 1)}}
	if !right.IsNull() {
		work = append(work, smtUTVPITerm{right, *big.NewRat(-1, 1)})
	}
	coefficients := make(map[ast.ASTHandle]*big.Rat)
	push := func(expression ast.ASTHandle, coefficient *big.Rat) {
		work = append(work, smtUTVPITerm{expression, *new(big.Rat).Set(coefficient)})
	}
	for len(work) > 0 {
		i := len(work) - 1
		term := work[i]
		work = work[:i]
		kind, err := l.manager.GetKind(term.expression)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		family, err := l.manager.GetFamilyID(term.expression)
		if err != nil {
			return false, err
		}
		count, err := l.manager.GetNumArgs(term.expression)
		if err != nil {
			return false, err
		}
		if family == l.family {
			operation, err := l.manager.GetDeclKind(term.expression)
			if err != nil {
				return false, err
			}
			switch operation {
			case smtArithOpAdd:
				for j := uint32(0); j < count; j++ {
					child, err := l.manager.GetArg(term.expression, j)
					if err != nil {
						return false, err
					}
					push(child, &term.coefficient)
				}
				continue
			case smtArithOpMul:
				if count == 2 {
					x, err := l.manager.GetArg(term.expression, 0)
					if err != nil {
						return false, err
					}
					y, err := l.manager.GetArg(term.expression, 1)
					if err != nil {
						return false, err
					}
					n, numeral, err := l.numeral(x)
					if err != nil {
						return false, err
					}
					if !numeral {
						x, y = y, x
						n, numeral, err = l.numeral(x)
						if err != nil {
							return false, err
						}
					}
					if numeral {
						push(y, new(big.Rat).Mul(&term.coefficient, n))
						continue
					}
				}
			case smtArithOpSub:
				if count == 2 {
					x, err := l.manager.GetArg(term.expression, 0)
					if err != nil {
						return false, err
					}
					y, err := l.manager.GetArg(term.expression, 1)
					if err != nil {
						return false, err
					}
					push(x, &term.coefficient)
					push(y, new(big.Rat).Neg(&term.coefficient))
					continue
				}
			case smtArithOpUMinus, smtArithOpToReal:
				if count == 1 {
					child, err := l.manager.GetArg(term.expression, 0)
					if err != nil {
						return false, err
					}
					coefficient := new(big.Rat).Set(&term.coefficient)
					if operation == smtArithOpUMinus {
						coefficient.Neg(coefficient)
					}
					push(child, coefficient)
					continue
				}
			}
			n, numeral, err := l.numeral(term.expression)
			if err != nil {
				return false, err
			}
			if numeral {
				l.weight.Add(&l.weight, new(big.Rat).Mul(n, &term.coefficient))
				continue
			}
		}
		if family != ast.NullFamilyID || count != 0 {
			return false, nil
		}
		if coefficients[term.expression] == nil {
			coefficients[term.expression] = new(big.Rat)
		}
		coefficients[term.expression].Add(coefficients[term.expression], &term.coefficient)
	}
	ids := make(map[ast.ASTHandle]uint32)
	for expression, coefficient := range coefficients {
		if coefficient.Sign() == 0 {
			continue
		}
		if coefficient.Cmp(big.NewRat(1, 1)) != 0 && coefficient.Cmp(big.NewRat(-1, 1)) != 0 {
			return false, nil
		}
		id, err := l.manager.GetASTID(expression)
		if err != nil {
			return false, err
		}
		ids[expression] = id
		l.terms = append(l.terms, smtUTVPITerm{expression, *new(big.Rat).Set(coefficient)})
		if len(l.terms) > 2 {
			return false, nil
		}
	}
	// Source obj_map traversal is unordered; keep the Go result deterministic.
	sort.Slice(l.terms, func(i, j int) bool { return ids[l.terms[i].expression] < ids[l.terms[j].expression] })
	return true, nil
}
