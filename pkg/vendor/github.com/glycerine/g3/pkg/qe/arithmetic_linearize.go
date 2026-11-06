package qe

// Source: src/qe/qe_arith.cpp imp::linearize/extract_coefficients.

import (
	"io"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/insdict"
)

func (c *arithmeticProjectionCall) linearizeTerm(multiplier *big.Rat, h ast.ASTHandle, constant *big.Rat, forms *[]ast.ASTRef, terms *insdict.Dict[ast.ASTHandle, *big.Rat]) error {
	p := c.owner
	m := p.manager
	args, mul, err := p.operation(h, p.family, 9)
	if err != nil {
		return err
	}
	if mul && len(args) == 2 {
		for i, arg := range args {
			v, ok, err := p.numeralExpression(arg)
			if err != nil {
				return err
			}
			if ok {
				return c.linearizeTerm(new(big.Rat).Mul(multiplier, v), args[1-i], constant, forms, terms)
			}
		}
	}
	args, add, err := p.operation(h, p.family, 6)
	if err != nil {
		return err
	}
	if add {
		for _, arg := range args {
			if err = c.linearizeTerm(multiplier, arg, constant, forms, terms); err != nil {
				return err
			}
		}
		return nil
	}
	args, sub, err := p.operation(h, p.family, 7)
	if err != nil {
		return err
	}
	if sub && len(args) == 2 {
		if err = c.linearizeTerm(multiplier, args[0], constant, forms, terms); err != nil {
			return err
		}
		return c.linearizeTerm(new(big.Rat).Neg(multiplier), args[1], constant, forms, terms)
	}
	args, neg, err := p.operation(h, p.family, 8)
	if err != nil {
		return err
	}
	if neg && len(args) == 1 {
		return c.linearizeTerm(new(big.Rat).Neg(multiplier), args[0], constant, forms, terms)
	}
	v, ok, err := p.numeral(h)
	if err != nil {
		return err
	}
	if ok {
		constant.Add(constant, new(big.Rat).Mul(multiplier, v))
		return nil
	}
	args, ite, err := p.operation(h, m.BasicFamilyID(), ast.ASTOpIte)
	if err != nil {
		return err
	}
	if ite && len(args) == 3 {
		value, err := c.eval(args[0])
		if err != nil {
			return err
		}
		truth := m.IsTrue(value)
		if !truth && !m.IsFalse(value) {
			return ast.ErrASTDomain
		}
		if err = p.emit("linearize", 188, func(w io.Writer) error {
			if e := writeExpression(m, p.config, w, args[0]); e != nil {
				return e
			}
			_, _ = io.WriteString(w, " := ")
			if e := writeExpression(m, p.config, w, value); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		}); err != nil {
			return err
		}
		if truth {
			if err = c.linearizeTerm(multiplier, args[1], constant, forms, terms); err != nil {
				return err
			}
			return p.appendRef(forms, args[0])
		}
		not, err := c.hold(m.MkNotSimplified(args[0]))
		if err != nil {
			return err
		}
		if err = p.appendRef(forms, not); err != nil {
			return err
		}
		return c.linearizeTerm(multiplier, args[2], constant, forms, terms)
	}
	args, mod, err := p.operation(h, p.family, 13)
	if err != nil {
		return err
	}
	if mod && len(args) == 2 {
		modulus, ok, err := p.numeralExpression(args[1])
		if err != nil {
			return err
		}
		if ok && modulus.Sign() != 0 {
			value, err := c.eval(h)
			if err != nil {
				return err
			}
			r, ok, err := p.numeral(value)
			if err != nil {
				return err
			}
			if !ok {
				return ast.ErrASTDomain
			}
			constant.Add(constant, new(big.Rat).Mul(multiplier, r))
			residue := new(big.Rat).Neg(r)
			nested := projectionTerms()
			if err = c.linearizeTerm(big.NewRat(1, 1), args[0], residue, forms, nested); err != nil {
				return err
			}
			coeffs, err := c.coefficients(nested)
			if err != nil {
				return err
			}
			return c.mbo.AddDivides(coeffs, residue, modulus)
		}
	}
	if err = p.emit("insert_mul", 42, func(w io.Writer) error {
		_, _ = io.WriteString(w, "Adding variable ")
		if e := writeExpression(m, p.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " "+multiplier.RatString()+"\n")
		return nil
	}); err != nil {
		return err
	}
	if old, ok := terms.Get2(h); ok {
		old.Add(old, multiplier)
	} else {
		terms.Set(h, new(big.Rat).Set(multiplier))
	}
	return nil
}

func (c *arithmeticProjectionCall) linearizeLiteral(lit ast.ASTHandle, forms *[]ast.ASTRef) (bool, error) {
	p := c.owner
	m := p.manager
	if ast.ASTSourceDebug {
		value, err := c.eval(lit)
		if err != nil {
			return false, err
		}
		if !m.IsTrue(value) {
			if err = p.emit("linearize", 65, func(w io.Writer) error {
				if e := writeExpression(m, p.config, w, lit); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " := ")
				if e := writeExpression(m, p.config, w, value); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); err != nil {
				return false, err
			}
			return false, ast.ErrASTDomain
		}
	}
	negative, inner, err := m.IsNot(lit)
	if err != nil {
		return false, err
	}
	if negative {
		lit = inner
	}
	if ast.ASTSourceDebug {
		double, _, e := m.IsNot(lit)
		if e != nil {
			return false, e
		}
		if double {
			return false, ast.ErrASTDomain
		}
	}
	mul := big.NewRat(1, 1)
	if negative {
		mul.Neg(mul)
	}
	terms := projectionTerms()
	constant := new(big.Rat)
	relation := simplex.LessEqual
	var left, right ast.ASTHandle
	matched := false
	for _, op := range []int32{2, 3, 4, 5} {
		args, ok, err := p.operation(lit, p.family, op)
		if err != nil {
			return false, err
		}
		if !ok || len(args) != 2 {
			continue
		}
		left, right = args[0], args[1]
		if op == 3 || op == 5 {
			left, right = right, left
		}
		if (op == 2 || op == 3) == negative {
			relation = simplex.Less
		}
		matched = true
		break
	}
	if !matched {
		args, eq, err := p.operation(lit, m.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return false, err
		}
		if eq && len(args) == 2 {
			_, arith, err := p.arithmetic(args[0])
			if err != nil {
				return false, err
			}
			if arith {
				left, right = args[0], args[1]
				relation = simplex.Equal
				matched = true
				if negative {
					lv, err := c.eval(left)
					if err != nil {
						return false, err
					}
					rv, err := c.eval(right)
					if err != nil {
						return false, err
					}
					l, ok, err := p.numeral(lv)
					if err != nil || !ok {
						return false, err
					}
					r, ok, err := p.numeral(rv)
					if err != nil || !ok {
						return false, err
					}
					if ast.ASTSourceDebug && l.Cmp(r) == 0 {
						return false, ast.ErrASTDomain
					}
					if l.Cmp(r) < 0 {
						left, right = right, left
					}
					relation = simplex.Less
				}
			}
		}
	}
	if !matched {
		args, distinct, err := p.operation(lit, m.BasicFamilyID(), ast.ASTOpDistinct)
		if err != nil {
			return false, err
		}
		if distinct && len(args) > 0 {
			_, arith, err := p.arithmetic(args[0])
			if err != nil {
				return false, err
			}
			if arith {
				if !negative {
					type entry struct {
						h     ast.ASTHandle
						value *big.Rat
					}
					values := make([]entry, 0, len(args))
					for _, arg := range args {
						v, e := c.eval(arg)
						if e != nil {
							return false, e
						}
						q, ok, e := p.numeral(v)
						if e != nil || !ok {
							return false, e
						}
						values = append(values, entry{arg, q})
					}
					sort.Slice(values, func(i, j int) bool { return values[i].value.Cmp(values[j].value) < 0 })
					for i := 0; i+1 < len(values); i++ {
						if ast.ASTSourceDebug && values[i].value.Cmp(values[i+1].value) >= 0 {
							return false, ast.ErrASTDomain
						}
						h, e := c.hold(m.MkFamilyApp(p.family, 4, nil, []ast.ASTHandle{values[i].h, values[i+1].h}))
						if e != nil {
							return false, e
						}
						ok, e := c.linearizeLiteral(h, forms)
						if e != nil || !ok {
							return false, e
						}
					}
					return true, nil
				}
				values := make(map[string]ast.ASTHandle)
				for _, arg := range args {
					v, e := c.eval(arg)
					if e != nil {
						return false, e
					}
					q, ok, e := p.numeral(v)
					if e != nil || !ok {
						return false, e
					}
					if other, ok := values[q.RatString()]; ok {
						left, right = arg, other
						matched = true
						relation = simplex.Equal
						break
					}
					values[q.RatString()] = arg
				}
				if !matched {
					if ast.ASTSourceDebug {
						return false, ast.ErrASTDomain
					}
					coeffs, e := c.coefficients(terms)
					if e != nil {
						return false, e
					}
					return true, c.mbo.AddConstraint(coeffs, constant, simplex.Equal)
				}
			}
		}
	}
	if !matched {
		err := p.emit("linearize", 146, func(w io.Writer) error {
			_, _ = io.WriteString(w, "Skipping ")
			if e := writeExpression(m, p.config, w, lit); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
		return false, err
	}
	if err = c.linearizeTerm(mul, left, constant, forms, terms); err != nil {
		return false, err
	}
	if err = c.linearizeTerm(new(big.Rat).Neg(mul), right, constant, forms, terms); err != nil {
		return false, err
	}
	coeffs, err := c.coefficients(terms)
	if err != nil {
		return false, err
	}
	return true, c.mbo.AddConstraint(coeffs, constant, relation)
}
