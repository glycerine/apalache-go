package qe

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (d *EqualityDER) negativeVariable(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	neg, err := d.operation(h, d.manager.BasicFamilyID(), ast.ASTOpNot)
	if err != nil || !neg {
		return ast.ASTHandle{}, false, err
	}
	v, err := d.manager.GetArg(h, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	yes, err := d.selection.IsVariable(v)
	return v, yes, err
}
func (d *EqualityDER) integer(h ast.ASTHandle) (bool, error) {
	s, err := d.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	integer, err := d.manager.MkFamilySort(d.arithmetic, 1, nil)
	return s == integer, err
}
func (d *EqualityDER) real(h ast.ASTHandle) (bool, error) {
	s, err := d.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	real, err := d.manager.MkFamilySort(d.arithmetic, 0, nil)
	return s == real, err
}
func (d *EqualityDER) invertibleConstant(integer bool, h ast.ASTHandle, coefficient *numeral.OwnedRational) (bool, error) {
	negative, err := d.operation(h, d.arithmetic, 8)
	if err != nil {
		return false, err
	}
	if negative {
		a, err := d.manager.GetArg(h, 0)
		if err != nil {
			return false, err
		}
		yes, err := d.invertibleConstant(integer, a, coefficient)
		if err != nil || !yes {
			return false, err
		}
		return true, d.manager.NumeralService().NegAssign(coefficient)
	}
	text, _, matched, err := d.manager.ArithmeticNumeralText(h)
	if err != nil || !matched {
		return false, err
	}
	n := d.manager.NumeralService()
	if err = n.SetString(coefficient, text); err != nil {
		return false, err
	}
	zero, err := n.IsZero(coefficient)
	if err != nil || zero {
		return false, err
	}
	if !integer {
		return true, nil
	}
	one, err := n.IsOne(coefficient)
	if err != nil || one {
		return one, err
	}
	return n.IsMinusOne(coefficient)
}
func (d *EqualityDER) invertibleProduct(integer bool, h ast.ASTHandle, coefficient *numeral.OwnedRational) (ast.ASTHandle, bool, error) {
	yes, err := d.selection.IsVariable(h)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if yes {
		return h, true, d.manager.NumeralService().SetInt64(coefficient, 1)
	}
	mul, err := d.operation(h, d.arithmetic, 9)
	if err != nil || !mul {
		return ast.ASTHandle{}, false, err
	}
	args, err := d.children(h)
	if err != nil || len(args) != 2 {
		return ast.ASTHandle{}, false, err
	}
	for _, pair := range [][2]ast.ASTHandle{{args[0], args[1]}, {args[1], args[0]}} {
		variable, err := d.selection.IsVariable(pair[0])
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if !variable {
			continue
		}
		invertible, err := d.invertibleConstant(integer, pair[1], coefficient)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if invertible {
			return pair[0], true, nil
		}
	}
	return ast.ASTHandle{}, false, nil
}

type signedEqualityTerm struct {
	positive bool
	term     ast.ASTHandle
}

func (d *EqualityDER) solveArithmetic(c *equalityDERCall, lhs, rhs ast.ASTHandle) ([]equalitySolution, bool, error) {
	integer, err := d.integer(lhs)
	if err != nil {
		return nil, false, err
	}
	if !integer {
		real, err := d.real(rhs)
		if err != nil || !real {
			return nil, false, err
		}
	}
	coefficient, err := c.rational()
	if err != nil {
		return nil, false, err
	}
	todo := []signedEqualityTerm{{true, lhs}, {false, rhs}}
	var done []signedEqualityTerm
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		add, err := d.operation(current.term, d.arithmetic, 6)
		if err != nil {
			return nil, false, err
		}
		if add {
			args, err := d.children(current.term)
			if err != nil {
				return nil, false, err
			}
			for _, arg := range args {
				todo = append(todo, signedEqualityTerm{current.positive, arg})
			}
			continue
		}
		variable, solved, err := d.invertibleProduct(integer, current.term, coefficient)
		if err != nil {
			return nil, false, err
		}
		if !solved {
			done = append(done, current)
			continue
		}
		done = append(done, todo...)
		inverse, err := c.rational()
		if err != nil {
			return nil, false, err
		}
		numbers := d.manager.NumeralService()
		if err = numbers.SetInt64(inverse, 1); err != nil {
			return nil, false, err
		}
		if err = numbers.DivAssign(inverse, coefficient); err != nil {
			return nil, false, err
		}
		var result []ast.ASTHandle
		for _, term := range done {
			factor, err := c.rational()
			if err != nil {
				return nil, false, err
			}
			if err = numbers.Copy(factor, inverse); err != nil {
				return nil, false, err
			}
			if current.positive == term.positive {
				if err = numbers.NegAssign(factor); err != nil {
					return nil, false, err
				}
			}
			h := term.term
			one, err := numbers.IsOne(factor)
			if err != nil {
				return nil, false, err
			}
			if !one {
				value, err := d.manager.ArithmeticNumeral(factor, integer)
				if err != nil {
					return nil, false, err
				}
				if value, err = c.hold(value); err != nil {
					return nil, false, err
				}
				h, err = d.manager.MkArithmeticMul(value, h)
				if err != nil {
					return nil, false, err
				}
				if h, err = c.hold(h); err != nil {
					return nil, false, err
				}
			}
			result = append(result, h)
		}
		var definition ast.ASTHandle
		if len(result) == 1 {
			kind, err := d.manager.GetKind(result[0])
			if err != nil {
				return nil, false, err
			}
			if kind == ast.ASTApp {
				definition = result[0]
			}
		}
		if definition.IsNull() {
			definition, err = d.manager.MkFamilyApp(d.arithmetic, 6, nil, result)
			if err != nil {
				return nil, false, err
			}
		}
		if definition, err = c.hold(definition); err != nil {
			return nil, false, err
		}
		err = d.emit("qe_lite", "solve_arith", 304, func(out io.Writer) error {
			for i, h := range []ast.ASTHandle{lhs, rhs, variable, definition} {
				if i != 0 {
					separator := " "
					if i == 3 {
						separator = " := "
					}
					_, _ = io.WriteString(out, separator)
				}
				if err := writeExpression(d.manager, d.config, out, h); err != nil {
					return err
				}
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
		return []equalitySolution{{variable, definition}}, err == nil, err
	}
	return nil, false, nil
}

func (d *EqualityDER) variableEquality(c *equalityDERCall, h ast.ASTHandle) ([]equalitySolution, bool, error) {
	eq, err := d.operation(h, d.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil {
		return nil, false, err
	}
	iff, err := d.operation(h, d.manager.BasicFamilyID(), ast.ASTOpIff)
	if err != nil {
		return nil, false, err
	}
	if eq || iff {
		args, err := d.children(h)
		if err != nil {
			return nil, false, err
		}
		left, right := args[0], args[1]
		lv, err := d.selection.IsVariable(left)
		if err != nil {
			return nil, false, err
		}
		rv, err := d.selection.IsVariable(right)
		if err != nil {
			return nil, false, err
		}
		boolean, err := d.booleanSort(left)
		if err != nil {
			return nil, false, err
		}
		if !lv && !rv && boolean {
			v, negative, err := d.negativeVariable(left)
			if err != nil {
				return nil, false, err
			}
			if !negative {
				left, right = right, left
				v, negative, err = d.negativeVariable(left)
				if err != nil {
					return nil, false, err
				}
			}
			if !negative {
				return nil, false, nil
			}
			term, err := d.manager.MkNot(right)
			if err != nil {
				return nil, false, err
			}
			term, err = c.hold(term)
			if err != nil {
				return nil, false, err
			}
			err = d.traceExpression("is_var_eq", 364, h)
			return []equalitySolution{{v, term}}, err == nil, err
		}
		if lv || rv {
			if !lv {
				left, right = right, left
			}
			if err = d.traceExpression("trivial_solve", 327, h); err != nil {
				return nil, false, err
			}
			return []equalitySolution{{left, right}}, true, nil
		}
		return d.solveArithmetic(c, left, right)
	}
	ite, err := d.operation(h, d.manager.BasicFamilyID(), ast.ASTOpIte)
	if err != nil {
		return nil, false, err
	}
	if ite {
		args, err := d.children(h)
		if err != nil {
			return nil, false, err
		}
		first, yes, err := d.variableEquality(c, args[1])
		if err != nil || !yes {
			return nil, false, err
		}
		second, yes, err := d.variableEquality(c, args[2])
		if err != nil || !yes || len(first) != len(second) {
			return nil, false, err
		}
		for i := range first {
			if first[i].variable != second[i].variable {
				return nil, false, nil
			}
		}
		for i := range first {
			term, err := d.manager.MkIte(args[0], first[i].term, second[i].term)
			if err != nil {
				return nil, false, err
			}
			first[i].term, err = c.hold(term)
			if err != nil {
				return nil, false, err
			}
		}
		return first, true, nil
	}
	variable, err := d.selection.IsVariable(h)
	if err != nil {
		return nil, false, err
	}
	if variable {
		term, err := d.manager.MkTrue()
		if err != nil {
			return nil, false, err
		}
		term, err = c.hold(term)
		if err != nil {
			return nil, false, err
		}
		err = d.traceExpression("is_var_eq", 396, h)
		return []equalitySolution{{h, term}}, err == nil, err
	}
	v, negative, err := d.negativeVariable(h)
	if err != nil || !negative {
		return nil, false, err
	}
	term, err := d.manager.MkFalse()
	if err != nil {
		return nil, false, err
	}
	term, err = c.hold(term)
	if err != nil {
		return nil, false, err
	}
	err = d.traceExpression("is_var_eq", 404, h)
	return []equalitySolution{{v, term}}, err == nil, err
}

func (d *EqualityDER) variableDefinition(c *equalityDERCall, h ast.ASTHandle, checkEquality bool) ([]equalitySolution, bool, error) {
	if checkEquality {
		return d.variableEquality(c, h)
	}
	negative, err := d.operation(h, d.manager.BasicFamilyID(), ast.ASTOpNot)
	if err != nil {
		return nil, false, err
	}
	// Preserve the source's actual argument here: is_var_eq(e), not its
	// unwrapped child. In particular, negated arithmetic equality is refused.
	if negative {
		return d.variableEquality(c, h)
	}
	result, yes, err := d.variableEquality(c, h)
	if err != nil || !yes || len(result) != 1 {
		return nil, false, err
	}
	boolean, err := d.booleanSort(result[0].variable)
	if err != nil || !boolean {
		return nil, false, err
	}
	rw, err := c.boolean()
	if err != nil {
		return nil, false, err
	}
	out, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return nil, false, err
	}
	if err = rw.MkNot(result[0].term, out); err != nil {
		return nil, false, err
	}
	result[0].term, err = d.manager.RefGet(out)
	return result, err == nil, err
}
