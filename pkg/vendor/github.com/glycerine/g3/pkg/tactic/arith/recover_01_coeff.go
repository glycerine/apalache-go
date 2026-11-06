package arith

// Source: recover_01_tactic.cpp find_zero_cls, find_coeff and process checks.

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

// recover01Coefficients validates the complete truth table before a caller
// constructs substitutions. Clause and literal handles are borrowed.
func recover01Coefficients(m *ast.ASTManager, clauses []ast.ASTHandle) ([]ast.ASTHandle, []big.Rat, bool, error) {
	if len(clauses) == 0 {
		return nil, nil, false, nil
	}
	n, e := m.GetNumArgs(clauses[len(clauses)-1])
	if e != nil {
		return nil, nil, false, e
	}
	if n < 2 || n > 31 {
		return nil, nil, false, nil
	}
	expected := uint64(1) << (n - 1)
	if uint64(len(clauses)) < expected {
		return nil, nil, false, nil
	}
	var zero ast.ASTHandle
	for _, c := range clauses {
		for i := uint32(0); i < n; i++ {
			lit, e := m.GetArg(c, i)
			if e != nil {
				return nil, nil, false, e
			}
			eq, e := arithmeticBasicApp(m, lit, ast.ASTOpEq)
			if e != nil {
				return nil, nil, false, e
			}
			if !eq {
				continue
			}
			a, e := m.GetArg(lit, 0)
			if e != nil {
				return nil, nil, false, e
			}
			b, e := m.GetArg(lit, 1)
			if e != nil {
				return nil, nil, false, e
			}
			for j := 0; j < 2; j++ {
				constant, e := arithmeticUninterpretedConstant(m, a)
				if e != nil {
					return nil, nil, false, e
				}
				text, _, num, e := m.ArithmeticNumeralText(b)
				if e != nil {
					return nil, nil, false, e
				}
				if constant && num {
					r, ok := new(big.Rat).SetString(text)
					if ok && r.Sign() == 0 {
						zero = c
						break
					}
				}
				a, b = b, a
			}
			if zero != (ast.ASTHandle{}) {
				break
			}
		}
		if zero != (ast.ASTHandle{}) {
			break
		}
	}
	if zero == (ast.ASTHandle{}) {
		return nil, nil, false, nil
	}
	var literals []ast.ASTHandle
	for i := uint32(0); i < n; i++ {
		lit, e := m.GetArg(zero, i)
		if e != nil {
			return nil, nil, false, e
		}
		eq, e := arithmeticBasicApp(m, lit, ast.ASTOpEq)
		if e != nil {
			return nil, nil, false, e
		}
		if !eq {
			literals = append(literals, lit)
		}
	}
	if len(literals) != int(n-1) {
		return nil, nil, false, nil
	}
	values := make([]big.Rat, int(expected))
	found := make([]bool, int(expected))
	for _, c := range clauses {
		size, e := m.GetNumArgs(c)
		if e != nil {
			return nil, nil, false, e
		}
		if size != n {
			return nil, nil, false, nil
		}
		idx := 0
		for bit, lit := range literals {
			matched := false
			for j := uint32(0); j < n; j++ {
				other, e := m.GetArg(c, j)
				if e != nil {
					return nil, nil, false, e
				}
				if other == lit {
					matched = true
					break
				}
				not, arg, e := m.IsNot(other)
				if e != nil {
					return nil, nil, false, e
				}
				if not && arg == lit {
					idx |= 1 << bit
					matched = true
					break
				}
				not, arg, e = m.IsNot(lit)
				if e != nil {
					return nil, nil, false, e
				}
				if not && arg == other {
					idx |= 1 << bit
					matched = true
					break
				}
			}
			if !matched {
				return nil, nil, false, nil
			}
		}
		var value *big.Rat
		for j := uint32(0); j < n && value == nil; j++ {
			lit, e := m.GetArg(c, j)
			if e != nil {
				return nil, nil, false, e
			}
			eq, e := arithmeticBasicApp(m, lit, ast.ASTOpEq)
			if e != nil {
				return nil, nil, false, e
			}
			if !eq {
				continue
			}
			for k := uint32(0); k < 2; k++ {
				a, e := m.GetArg(lit, k)
				if e != nil {
					return nil, nil, false, e
				}
				text, _, ok, e := m.ArithmeticNumeralText(a)
				if e != nil {
					return nil, nil, false, e
				}
				if ok {
					value, _ = new(big.Rat).SetString(text)
					break
				}
			}
		}
		if value == nil || (found[idx] && values[idx].Cmp(value) != 0) {
			return nil, nil, false, nil
		}
		found[idx] = true
		values[idx].Set(value)
	}
	for idx := range values {
		if !found[idx] {
			return nil, nil, false, nil
		}
		var sum big.Rat
		for bit := range literals {
			if idx&(1<<bit) != 0 {
				sum.Add(&sum, &values[1<<bit])
			}
		}
		if sum.Cmp(&values[idx]) != 0 {
			return nil, nil, false, nil
		}
	}
	coefficients := make([]big.Rat, len(literals))
	for bit := range literals {
		coefficients[bit].Set(&values[1<<bit])
	}
	return literals, coefficients, true, nil
}
