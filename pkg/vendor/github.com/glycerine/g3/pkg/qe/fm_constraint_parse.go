package qe

// Source: src/qe/qe_lite.cpp fm::fm variable/literal registration and add_constraint.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *fmStore) variable(h ast.ASTHandle) (result uint32, err error) {
	defer func() {
		if err != nil {
			return
		}
		r := s.recognizer
		err = r.emit("to_var", 1677, func(out io.Writer) error {
			if err := writeExpression(r.manager, r.config, out, h); err != nil {
				return err
			}
			_, _ = io.WriteString(out, " --> "+strconv.FormatUint(uint64(result), 10)+"\n")
			return nil
		})
	}()
	if x, ok := s.variableIndex[h]; ok {
		return x, nil
	}
	r := s.recognizer
	m := r.manager
	i, err := s.integer(h)
	if err != nil {
		return 0, err
	}
	sort, err := m.GetSort(h)
	if err != nil {
		return 0, err
	}
	real, err := m.MkFamilySort(r.arithmetic, 0, nil)
	if err != nil {
		return 0, err
	}
	if !i && sort != real {
		return 0, ast.ErrASTDomain
	}
	index, err := m.GetVarIndex(h)
	if err != nil {
		return 0, err
	}
	ref, err := m.NewRef(h)
	if err != nil {
		return 0, err
	}
	x := uint32(len(s.variables))
	s.variables = append(s.variables, fmVariable{expression: ref, integer: i, forbidden: r.forbidden[index] || (r.realOnly && i)})
	s.variableIndex[h] = x
	err = r.emit("mk_var", 1655, func(out io.Writer) error {
		if err := writeExpression(m, r.config, out, h); err != nil {
			return err
		}
		flag := "0"
		if s.variables[x].forbidden {
			flag = "1"
		}
		_, _ = io.WriteString(out, " |-> "+strconv.FormatUint(uint64(x), 10)+" forbidden: "+flag+"\n")
		return nil
	})
	return x, err
}
func (s *fmStore) boolean(h ast.ASTHandle) (int32, error) {
	if x, ok := s.booleanIndex[h]; ok {
		return x, nil
	}
	r := s.recognizer
	m := r.manager
	yes, err := r.uninterpretedConstant(h)
	if err != nil {
		return 0, err
	}
	if !yes {
		return 0, ast.ErrASTDomain
	}
	sort, err := m.GetSort(h)
	if err != nil {
		return 0, err
	}
	b, err := m.MkBoolSort()
	if err != nil {
		return 0, err
	}
	if sort != b {
		return 0, ast.ErrASTDomain
	}
	if len(s.booleans) >= 1<<31 {
		return 0, ast.ErrASTDomain
	}
	ref, err := m.NewRef(h)
	if err != nil {
		return 0, err
	}
	x := int32(len(s.booleans))
	s.booleans = append(s.booleans, ref)
	s.booleanIndex[h] = x
	return x, nil
}
func (s *fmStore) literal(h ast.ASTHandle) (int32, error) {
	r := s.recognizer
	negative, err := r.operation(h, r.manager.BasicFamilyID(), ast.ASTOpNot)
	if err != nil {
		return 0, err
	}
	if negative {
		h, err = r.manager.GetArg(h, 0)
		if err != nil {
			return 0, err
		}
	}
	x, err := s.boolean(h)
	if negative {
		x = -x
	}
	return x, err
}
func (s *fmStore) numeral(scope *fmScope, h ast.ASTHandle) (*numeral.OwnedRational, error) {
	n := s.recognizer.manager.NumeralService()
	text, _, yes, err := s.recognizer.manager.ArithmeticNumeralText(h)
	if err != nil {
		return nil, err
	}
	if !yes {
		return nil, ast.ErrASTDomain
	}
	v, err := scope.number()
	if err != nil {
		return nil, err
	}
	return v, n.SetString(v, text)
}

func (s *fmStore) parse(form ast.ASTHandle) (result *fmConstraint, err error) {
	scope, err := s.begin()
	if err != nil {
		return nil, err
	}
	defer s.finish(scope, &err)
	r := s.recognizer
	m := r.manager
	n := m.NumeralService()
	if err = r.emit("add_constraint", 1697, func(out io.Writer) error {
		if err := writeExpression(m, r.config, out, form); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	}); err != nil {
		return nil, err
	}
	args := []ast.ASTHandle{form}
	disjunction, err := r.operation(form, m.BasicFamilyID(), ast.ASTOpOr)
	if err != nil {
		return nil, err
	}
	if disjunction {
		if !r.occ {
			return nil, ast.ErrASTDomain
		}
		count, err := m.GetNumArgs(form)
		if err != nil {
			return nil, err
		}
		args = make([]ast.ASTHandle, count)
		for i := range args {
			args[i], err = m.GetArg(form, uint32(i))
			if err != nil {
				return nil, err
			}
		}
	}
	var lits []int32
	var xs []uint32
	var as []*numeral.OwnedRational
	c, err := scope.number()
	if err != nil {
		return nil, err
	}
	strict, found := false, false
	for _, arg := range args {
		literal, err := r.literal(arg)
		if err != nil {
			return nil, err
		}
		if literal {
			l, err := s.literal(arg)
			if err != nil {
				return nil, err
			}
			lits = append(lits, l)
			continue
		}
		if found {
			return nil, ast.ErrASTDomain
		}
		found = true
		negative, err := r.operation(arg, m.BasicFamilyID(), ast.ASTOpNot)
		if err != nil {
			return nil, err
		}
		if negative {
			arg, err = m.GetArg(arg, 0)
			if err != nil {
				return nil, err
			}
		}
		strict = negative
		ge, err := r.operation(arg, r.arithmetic, 3)
		if err != nil {
			return nil, err
		}
		le, err := r.operation(arg, r.arithmetic, 2)
		if err != nil {
			return nil, err
		}
		if !ge && !le {
			return nil, ast.ErrASTDomain
		}
		if ge {
			negative = !negative
		}
		lhs, err := m.GetArg(arg, 0)
		if err != nil {
			return nil, err
		}
		rhs, err := m.GetArg(arg, 1)
		if err != nil {
			return nil, err
		}
		rhsNumber, err := s.numeral(scope, rhs)
		if err != nil {
			return nil, err
		}
		if err = n.Copy(c, rhsNumber); err != nil {
			return nil, err
		}
		if negative {
			if err = n.NegAssign(c); err != nil {
				return nil, err
			}
		}
		mons := []ast.ASTHandle{lhs}
		add, err := r.operation(lhs, r.arithmetic, 6)
		if err != nil {
			return nil, err
		}
		if add {
			count, err := m.GetNumArgs(lhs)
			if err != nil {
				return nil, err
			}
			mons = make([]ast.ASTHandle, count)
			for i := range mons {
				mons[i], err = m.GetArg(lhs, uint32(i))
				if err != nil {
					return nil, err
				}
			}
		}
		allInt := true
		for _, mon := range mons {
			a, err := scope.number()
			if err != nil {
				return nil, err
			}
			x := mon
			mul, err := r.operation(mon, r.arithmetic, 9)
			if err != nil {
				return nil, err
			}
			if mul {
				count, err := m.GetNumArgs(mon)
				if err != nil {
					return nil, err
				}
				if count != 2 {
					return nil, ast.ErrASTDomain
				}
				coefficient, err := m.GetArg(mon, 0)
				if err != nil {
					return nil, err
				}
				v, err := s.numeral(scope, coefficient)
				if err != nil {
					return nil, err
				}
				if err = n.Copy(a, v); err != nil {
					return nil, err
				}
				x, err = m.GetArg(mon, 1)
				if err != nil {
					return nil, err
				}
			} else {
				if err = n.SetInt64(a, 1); err != nil {
					return nil, err
				}
			}
			if negative {
				if err = n.NegAssign(a); err != nil {
					return nil, err
				}
			}
			x, selected, err := r.variable(x)
			if err != nil {
				return nil, err
			}
			if !selected {
				return nil, ast.ErrASTDomain
			}
			index, err := s.variable(x)
			if err != nil {
				return nil, err
			}
			xs = append(xs, index)
			as = append(as, a)
			if !s.variables[index].integer {
				allInt = false
			}
		}
		if err = s.makeIntegral(scope, as, c); err != nil {
			return nil, err
		}
		if allInt && strict {
			strict = false
			if err = n.AddAssignInt(c, -1); err != nil {
				return nil, err
			}
		}
	}
	if !found {
		return nil, ast.ErrASTDomain
	}
	if err = s.traceVariables(xs); err != nil {
		return nil, err
	}
	result, err = s.newConstraint(lits, xs, as, c, strict)
	if err != nil {
		return nil, err
	}
	if err = s.traceConstraint("add_constraint", 1788, "add_constraint: ", result); err != nil {
		return nil, err
	}
	return result, nil
}
